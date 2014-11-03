require 'rspec'
require '../stressor'

class FakeNATSClient
  def initialize
    @interactions = []
  end

  def subscribe(subject, &blk)
    @interactions << [:subscribe, subject]
    blk.call("broadcast", nil, subject)
  end

  def publish(subject, message)
    @interactions << [:publish, subject, message]
  end

  def request(subject, data, &blk)
    @interactions << [:request, subject, data]
    blk.call('request_reply')
  end

  attr_reader :interactions
end

describe "Nats Stressor" do
  it "sets up subscriptions and publishes" do
    client = FakeNATSClient.new
    logger = double(:logger)

    allow(logger).to receive(:info)

    stressor = NATSStressor.new(client, logger, "stressor1")

    stressor.start
    expect(client.interactions).to eq([[:subscribe, '*']])
    expect(logger).to have_received(:info).with("nats.broadcast.received", "broadcast")

    stressor.perform_interactions
    p client.interactions

    p logger
    #expect(client.interactions).to eq([[:subscribe, '*'], [:publish, 'stressor1.pub', 'pubmessage'], [:request, 'stressor1.req', 'reqmessage']])
    #expect(logger).to have_received(:info).with("nats.request", "request_reply")
  end
end


