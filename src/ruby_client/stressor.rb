require 'socket'
require "net/http"

class NATSStressor
  def initialize(client, logger, name, payload_size, population, api_key, storage_file, socket)
    @client = client
    @logger = logger
    @name = name
    @payload_size = payload_size
    @msg_counter = 0
    @api_key = api_key
    @storage_file = storage_file
    @socket = socket
  end

  def start
    @client.subscribe(">") do |message, reply, subject|
      communicate_metric("received---" + message)
      if message =~ /^publish--/
        @client.publish("ruby.publish", "received_publish--#{@name}--#{message}")
      end
    end
  end

  def perform_interactions
    #request_msg = "request--#{@name}--#{@msg_counter}--" + "."*@payload_size
    publish_msg = "publish--#{@name}--#{@msg_counter}--" + "."*@payload_size

    @client.publish("ruby.publish", publish_msg)
    communicate_metric("sent---" + publish_msg)

    File.write(@storage_file, JSON.pretty_generate(@message_tally))
    # @client.request("ruby.request", request_msg) do |response|
    #   @logger.info("receiving_response #{request_msg} #{response}")
    # end
    @msg_counter += 1
  end

  private
  def communicate_metric(message)
    http = Net::HTTP.new('127.0.0.1', 4568)
    request = Net::HTTP::Post.new("/messages")
    request.body = message
    http.request(request)
    #UNIXSocket.new(@socket).tap{ |s| s.puts message }.close
  rescue Errno::ECONNREFUSED => e
    @logger.info("couldn't reach metrics")
    puts e.backtrace
  end
end
