#!/usr/bin/env ruby
require 'dogapi'
require 'yaml'
require 'json'
require 'faraday'
require_relative './lib'

#####################################################################

require 'dogapi'
require 'yaml'
require 'sinatra'
require 'thread'
require 'uri'
require 'json'

class LoggerDog
  def initialize(dog)
    @dog = dog
  end

  def method_missing(meth, *args, &blk)
    # puts "sending: #{meth}, #{args.join(',')}"
    @dog.send(meth, *args, &blk)
  end
end

class FakeDog
  def initialize(*whatever)

  end
  def batch_metrics(&blk)
    blk.call
  end
  def emit_point(k,v, hsh={})
    p({k: k, v: v, hsh: hsh})
  end
end

config = YAML.load_file(ARGV[0])
datadog_api_key = config["datadog_api_key"]
datadog = LoggerDog.new(Dogapi::Client.new(datadog_api_key))
name = config["name"]

sleep 5.0

loop do
  conn = Faraday.new(:url => config["nats_monitor_uri"]) do |faraday|
    faraday.request  :url_encoded             # form-encode POST params
    faraday.adapter  Faraday.default_adapter  # make requests with Net::HTTP
  end

  varz = Lib.new.flat_hash('nats_stress.nats_monitor.varz', JSON.parse(conn.get('/varz').body))
  subscriptionsz = Lib.new.flat_hash('nats_stress.nats_monitor.subz', JSON.parse(conn.get('/subscriptionsz').body))

  connz = JSON.parse(conn.get('/connz').body)

  datadog.batch_metrics do
    varz.each do |k,v|
      datadog.emit_point(k,v,host: name)
    end
    subscriptionsz.each do |k,v|
      datadog.emit_point(k,v,host: name)
    end

    p connz
    conns = connz.delete('connections')
    p conns
    connz_top = Lib.new.flat_hash('nats_stress.nats_monitor.conz', connz)
    connz_top.each do |k,v|
      datadog.emit_point(k,v,host: name)
    end

    conns.each do |conn|
      conn_flat = Lib.new.flat_hash('nats_stress.nats_monitor.conn', conn)
      conn_flat.each do |k,v|
        datadog.emit_point(k,v,host: name, tags: ["client:#{conn['ip']}"])
      end
    end
  end

  sleep 0.9
end


