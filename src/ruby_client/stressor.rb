require 'socket'

class NATSStressor
  def initialize(client, logger, name, payload_size, population, api_key, storage_file, socket)
    @client = client
    @logger = logger
    @name = name
    @payload_size = payload_size
    @msg_counter = 0
    @message_tally = {}
    @msgs_completed = 0
    @population = population
    @api_key = api_key
    @storage_file = storage_file
    @socket = socket
  end

  def start
    @client.subscribe(">") do |message, reply, subject|
      if message =~ /^publish--/
        @client.publish("ruby.publish", "received_publish--#{@name}--#{message}")
      end
      if message =~ /^received_publish--.*?--publish--#{@name}/
        original_msg = message[/^received_publish--(.*?)--(.*)/, 2]
        puts "                : #{message}"
        puts "original_msg rec: #{original_msg}"
        recepient = message[/^received_publish--(.*?)--(.*)/, 1]
        @message_tally[original_msg] ||= []
        @message_tally[original_msg] << recepient
        if @message_tally[original_msg].uniq.size == @population
          @logger.info "tallied up #{@message_tally[original_msg].join(",")}"
          @msgs_completed += 1
          @message_tally.delete(original_msg)
        end
      end
    end
  end

  def perform_interactions
    #request_msg = "request--#{@name}--#{@msg_counter}--" + "."*@payload_size
    publish_msg = "publish--#{@name}--#{@msg_counter}--" + "."*@payload_size

    @logger.info("publishing " + publish_msg)
    @logger.info("completed #{@msgs_completed} messsages. outstanding: #{@message_tally.keys.size}")
    puts "original_msg pub: #{publish_msg}"
    @client.publish("ruby.publish", publish_msg)
    communicate_metrics(publish_msg)
    @message_tally[publish_msg] = []

    File.write(@storage_file, JSON.pretty_generate(@message_tally))
    #@logger.info("requesting " + request_msg)
    #@client.request("ruby.request", request_msg) do |response|
      #@logger.info("receiving_response #{request_msg} #{response}")
    #end
    @msg_counter += 1
  end

  def upload_data
    datadog("msgs.sent", @msg_counter)
    datadog("msgs.completed", @msgs_completed)
    datadog("msgs.outstanding", @message_tally.keys.size)
  end

  private
  def communicate_metrics(message)
    UNIXSocket.new(@socket).tap{ |s| s.puts message }.close
  end

  def datadog(metric, value)
    curl = <<-BASH
      curl -s -X POST -H "Content-type: application/json" \
      -d '{ "series" :
               [{"metric":"nats-stress.#{metric}",
                "points":[[#{Time.now.utc.to_i}, #{value}]],
                "type":"gauge",
                "host":"#{@name}",
                "tags":["client:ruby"]}
              ]
          }' \
      'https://app.datadoghq.com/api/v1/series?api_key=#{@api_key}'
    BASH
    `#{curl}`
  end
end
