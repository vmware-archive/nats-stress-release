#!/usr/bin/env ruby

require 'dogapi'
require 'yaml'
require 'socket'
require 'thread'
require 'uri'

class Metrics
  def initialize(config)
    @name = config["name"]
    datadog_api_key = config["datadog_api_key"]
    @datadog = Dogapi::Client.new(datadog_api_key)
    @servers = config["nats_servers"]

    @client = case @name
              when /ruby/ then 'ruby'
              when /yagnats_og/ then 'yagnats-og'
              when /yagnats_apcera/ then 'yagnats-apcera'
              end

    @message_count = 0
    @last_timestamp = Time.now
    @mutex = Mutex.new
  end

  def message_sent!
    @mutex.synchronize do
      @message_count += 1
    end
  end

  def reset_messages_counter!
    @mutex.synchronize do
      @message_count = 0
    end
  end

  def message_rate
    @message_count / seconds_passed
  end

  def seconds_passed
    Time.now - @last_timestamp
  end

  def upload
    nats_connection = `lsof -iTCP | grep 4222 | grep -v root`
    server = /(?<server>\d+\.\d+\.\d+\.\d+):4222/.match(nats_connection)[:server]
    @servers.each do |s|
      uri = URI(s)
      @datadog.emit_point("nats-stress.server-#{uri.host}", uri.host == server ? 1 : 0, host: @name, tags: ["client:#{@client}"])
    end
    @datadog.emit_point("nats-stress.msgs.sent", @message_count, host: @name, tags: ["client:#{@client}"])
    @datadog.emit_point("nats-stress.msgs.send_rate", message_rate, host: @name, tags: ["client:#{@client}"])
    @last_timestamp = Time.now
    reset_messages_counter!
  rescue => e
    puts "can't upload: ", e
  end
end

conf = YAML.load_file(ARGV[0])
metrics = Metrics.new(conf)

threads = []

threads << Thread.new do
  loop do
    metrics.upload
    sleep 1
  end
end


threads << Thread.new do
  begin
    file = conf['socket']
    File.unlink(file) if File.exists?(file) && File.socket?(file)
    server = UNIXServer.new(file)
    loop do
      message = server.accept.read
      metrics.message_sent!
    end
  rescue => e
    puts e
  end
end

threads.each(&:join)
