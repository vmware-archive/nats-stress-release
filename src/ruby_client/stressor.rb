class NATSStressor
  def initialize(client, logger, name, payload_size)
    @client = client
    @logger = logger
    @name = name
    @payload_size = payload_size
    @msg_counter = 1
  end

  def start
    @client.subscribe(">") do |message, reply, subject|
      @logger.info("nats.broadcast.received", {message: message})
    end
  end

  def perform_interactions
    uid = @name + @msg_counter.to_s
    request_msg = "request_#{uid}" + "."*@payload_size
    publish_msg = "publish_#{uid}" + "."*@payload_size

    @client.publish("broadcasts_of_#{@name}", publish_msg)
    @logger.info("nats.request.sent", {message: "requesting " + request_msg})
    @client.request("requests_of_#{@name}", request_msg) do |response|
      @logger.info("nats.request_reply.received", {message: "got_response for #{request_msg}. it is #{response}"})
    end
    @msg_counter += 1
  end
end
