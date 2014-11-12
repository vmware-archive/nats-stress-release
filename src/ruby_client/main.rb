#!/usr/bin/env ruby

require 'nats/client'
require 'steno'
require 'yaml'
require_relative './stressor'

conf = YAML.load_file(ARGV[0])

EM.run do
  client = NATS.connect(
    :uri => conf["nats_servers"],
    :max_reconnect_attempts => Float::INFINITY,
    :dont_randomize_servers => true,
  )
  config = Steno::Config.from_hash(
    :file => "/var/vcap/sys/log/ruby_client/ruby_client.log",
    :level => "debug"
  )
  Steno.init(config)
  logger = Steno.logger("ruby.client")

  stressor = NATSStressor.new(client, logger, conf["name"], conf["payload_size_in_bytes"], conf["population"], conf["datadog_api_key"],
                             conf["storage_file"])
  stressor.start

  EM.add_periodic_timer(conf["publish_interval_in_seconds"]) do
    stressor.perform_interactions
  end
end
