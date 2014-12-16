#!/usr/bin/env ruby
require 'dogapi'
require 'yaml'
require 'sinatra'
require 'thread'
require 'uri'
require 'json'

class Metrics
  def initialize(config)
    @name = config["name"]
    @population = config["population"]
    datadog_api_key = config["datadog_api_key"]
    @datadog = Dogapi::Client.new(datadog_api_key)
    @servers = config["nats_servers"]

    @client = case @name
              when /ruby/ then 'ruby'
              when /yagnats_og/ then 'yagnats-og'
              when /yagnats_apcera/ then 'yagnats-apcera'
              end

    @message_tally = {}
    @totals = {
      complete: 0,
      sent: 0,
      received: 0,
    }
    @counts = {
      complete: 0,
      sent: 0,
      received: 0,
    }
    @last_timestamp = Time.now
    @mutex = Mutex.new
  end

  def processes(recv, sent)
    #puts "!!!!"
    #puts recv
    #puts sent
    message_received!(recv)
    message_sent!(sent)
  end
  
  def process(message)
    if message =~ /^received---received_publish--.*?--publish--#{@name}/
      original_msg = message[/^received---received_publish--(.*?)--(.*)/, 2]
      recepient = message[/^received---received_publish--(.*?)--(.*)/, 1]
      receipts = add_receipt(original_msg, recepient)
      if receipts.size == @population
        message_complete!(message)
      end
    elsif message =~ /^sent---publish--#{@name}/
      message_sent_incr!
    elsif message =~ /^received---publish/
      message_received_incr!
    end
  end

  def add_receipt(message, recepient)
    @mutex.synchronize do
      @message_tally[message] ||= []
      @message_tally[message] << recepient
      @message_tally[message].uniq!
    end
    @message_tally[message]
  end

  def incr!(bucket, amount)
    @mutex.synchronize do
      @totals[bucket] += amount
      @counts[bucket] += amount
    end
  end

  def message_complete!(message)
    incr!(:complete)
    @mutex.synchronize do
      @message_tally.delete(message)
    end
  end

  def message_sent_incr!
    incr!(:sent, 1)
  end

  def message_received_incr!
    incr!(:received, 1)
  end

  def message_sent!(amount)
    incr!(:sent, amount)
  end

  def message_received!(amount)
    incr!(:received, amount)
  end

  def reset_messages_counter!
    @mutex.synchronize do
      @counts = {
        complete: 0,
        sent: 0,
        received: 0,
      }
    end
  end

  def seconds_passed
    Time.now - @last_timestamp
  end

  def rate(bucket, counts)
    counts[bucket] / seconds_passed
  end

  def total(bucket, totals)
    totals[bucket]
  end

  def upload
    return if seconds_passed < 1.000
    nats_connection = `lsof -iTCP | grep 4222 | grep -v root`
    server = /(?<server>\d+\.\d+\.\d+\.\d+|localhost):4222/.match(nats_connection)[:server]
    @servers.each do |s|
      uri = URI(s)
      @datadog.emit_point("nats-stress.server-#{uri.host}", uri.host == server ? 1 : 0, host: @name, tags: ["client:#{@client}"])
    end

    counts = @counts.dup
    totals = @totals.dup
    complete_total = total(:complete, totals)
    complete_rate  = rate(:complete, counts)
    sent_total     = total(:sent, totals)
    sent_rate      = rate(:sent, counts)
    received_total = total(:received, totals)
    received_rate  = rate(:received, counts)

    #puts "***"
    #puts sent_total
    #puts received_total
    
    @datadog.emit_point("nats-stress.msgs.complete", complete_total, host: @name, tags: ["client:#{@client}"])
    @datadog.emit_point("nats-stress.msgs.complete_rate", complete_rate, host: @name, tags: ["client:#{@client}"])

    @datadog.emit_point("nats-stress.msgs.sent", sent_total, host: @name, tags: ["client:#{@client}"])
    @datadog.emit_point("nats-stress.msgs.send_rate", sent_rate, host: @name, tags: ["client:#{@client}"])

    @datadog.emit_point("nats-stress.msgs.received", received_total, host: @name, tags: ["client:#{@client}"])
    @datadog.emit_point("nats-stress.msgs.received_rate", received_rate, host: @name, tags: ["client:#{@client}"])

    @last_timestamp = Time.now
    reset_messages_counter!
  rescue => e
    puts "can't upload: ", e
    puts e.backtrace
  end
end

conf = YAML.load_file(ARGV[0])
metrics = Metrics.new(conf)

upload_thread = Thread.new do
  loop do
    metrics.upload
    sleep 0.97
  end
end

set :server, :puma
set :bind, '0.0.0.0'
set :port, 4568

disable :logging

get '/' do
  'jello world'
end

post '/messages' do
  message = request.body.read
  metrics.process(message)
  "bye"
end

post '/messages-new' do
  message = request.body.read
  pair = message.split(",")  
  metrics.processes(pair[0].to_i, pair[1].to_i)
  "bye"
end
