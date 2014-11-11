class NATSStressor
  def initialize(client, logger, name, payload_size)
    @client = client
    @logger = logger
    @name = name
    @payload_size = payload_size
    @msg_counter = 1
    @received = {}
  end

  def start
    @client.subscribe(">") do |message, reply, subject|
      type, name, _ = message.split('--')
      if type == 'publish'
        @received[name] ||= 0
        @received[name] += 1
        @logger.info(@received.map { |k, v| "#{k}: #{v}" }.join(' '))
      end
      # @logger.info("receiving " + message)
    end
  end

  def perform_interactions
    request_msg = "request--#{@name}--#{@msg_counter}--" + "."*@payload_size
    publish_msg = "publish--#{@name}--#{@msg_counter}--" + "."*@payload_size

    @logger.info("publishing " + publish_msg)
    @client.publish("ruby.publish", publish_msg)

    @logger.info("requesting " + request_msg)
    @client.request("ruby.request", request_msg) do |response|
      @logger.info("receiving_response #{request_msg} #{response}")
    end
    @msg_counter += 1
  end
end
