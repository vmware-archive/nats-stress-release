require 'rspec'
require 'json'
require_relative './lib'

describe "stuff" do
  it "asdf" do
    hash = JSON.parse(<<-HERE
      {
        "stats": {
          "num_subscriptions": 1,
          "num_cache": 1,
          "num_inserts": 1,
          "num_removes": 0,
          "num_matches": 386,
          "cache_hit_rate": 0.9974093264248705,
          "max_fanout": 1,
          "avg_fanout": 1,
          "stats_time": "2014-11-26T10:04:43.778839246-08:00"
        }
      }
                      HERE
                     )
    expect(Lib.new.flat_hash("", hash)[".stats.num_removes"]).to eq(0)
  end

  it "few" do
    h = { :a => { :b => [
      {key: 'foo', :c => 7},
      {key: 'bar', :d => 2 }
    ],
    :e => 3 },
    :f => 4 }

    new_h = Lib.new.transform(h) do |k,v|
      if v.is_a? Array
        h = {}
        v.each do |e|
          h[e[:key]] = e
        end
        [k,h]
      else
        [k,v]
      end
    end
    p new_h
    expect(new_h[:a][:b]['foo'][:c]).to eq(7)
  end

  it "real hash" do
    hash = JSON.parse(<<-HERE
{
  "num_connections": 1,
  "offset": 0,
  "limit": 100,
  "connections": [
    {
      "cid": 1,
      "ip": "127.0.0.1",
      "port": 55798,
      "pending_size": 0,
      "in_msgs": 796,
      "out_msgs": 796,
      "in_bytes": 24566,
      "out_bytes": 24566,
      "subscriptions": 1
    }
  ]
}
                      HERE
                     )
    new_h = Lib.new.transform(hash) do |k,v|
      if v.is_a? Array
        h = {}
        v.each do |e|
          h[e['ip']] = e
        end
        [k,h]
      else
        [k,v]
      end
    end
    new_h = Lib.new.flat_hash('nats', new_h)
    expect(new_h['nats.connections.127.0.0.1.port']).to eq(55798)
  end
end
