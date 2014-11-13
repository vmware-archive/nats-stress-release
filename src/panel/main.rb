require 'sinatra'

set :bind, '0.0.0.0'

NATS_IPS = [
  '10.10.19.101',
  '10.10.81.101',
]

def run_commands(commands)
  output = {}
  commands.each do |command|
    output[command] = `#{command} 2>&1`
  end
  result = ""
  result << "Time: #{Time.now}\n"
  output.each do |c, o|
    result << c << "\n"
    result << o << "\n"
  end
  result
end

get '/' do
  "jello world, it's delicious #{`whoami`}"
end

get '/ifconfig' do
  `ifconfig`
end

get '/slowmo/:interface/:port/:time' do
  commands = [
    "tc qdisc del dev #{params[:interface]} root",
    "tc qdisc add dev #{params[:interface]} handle 1: root htb",
    "tc class add dev #{params[:interface]} parent 1: classid 1:11 htb rate 4gbps",
    "tc qdisc add dev #{params[:interface]} parent 1:11 netem delay #{params[:time]}",
    "tc filter add dev #{params[:interface]} protocol ip prio 1 u32 match ip dport #{params[:port]} 0xffff flowid 1:11",
  ]
  run_commands(commands)
end

get '/dropmo/:interface/:port/:rate' do
  commands = [
    "tc qdisc del dev #{params[:interface]} root",
    "tc qdisc add dev #{params[:interface]} handle 1: root htb",
    "tc class add dev #{params[:interface]} parent 1: classid 1:11 htb rate 4gbps",
    "tc qdisc add dev #{params[:interface]} parent 1:11 netem loss #{params[:rate].to_f * 100.0}%",
    "tc filter add dev #{params[:interface]} protocol ip prio 1 u32 match ip dport #{params[:port]} 0xffff flowid 1:11",
  ]
  run_commands(commands)
end

get '/slowmo_client/:interface/:time' do
  commands = [
    "tc qdisc del dev #{params[:interface]} root",
    "tc qdisc add dev #{params[:interface]} handle 1: root htb",
    "tc class add dev #{params[:interface]} parent 1: classid 1:11 htb rate 4gbps",
    "tc qdisc add dev #{params[:interface]} parent 1:11 netem delay #{params[:time]}",
  ]
  NATS_IPS.each do |ip|
    commands << "tc filter add dev #{params[:interface]} protocol ip prio 1 u32 match ip dst #{ip} flowid 1:11"
  end
  run_commands(commands)
end

get '/dropmo_client/:interface/:rate' do
  commands = [
    "tc qdisc del dev #{params[:interface]} root",
    "tc qdisc add dev #{params[:interface]} handle 1: root htb",
    "tc class add dev #{params[:interface]} parent 1: classid 1:11 htb rate 4gbps",
    "tc qdisc add dev #{params[:interface]} parent 1:11 netem loss #{params[:rate].to_f * 100.0}%",
  ]
  NATS_IPS.each do |ip|
    commands << "tc filter add dev #{params[:interface]} protocol ip prio 1 u32 match ip dst #{ip} flowid 1:11"
  end
  run_commands(commands)
end

get '/reset/:interface' do
  commands = [
    "tc qdisc del dev #{params[:interface]} root",
  ]
  run_commands(commands)
end


get '/download_logs' do
  ['/var/vcap/sys/log/ruby_client/ruby_client.log',
   '/var/vcap/sys/log/yagnats_apcera_client/yagnats_apcera_client.log',
   '/var/vcap/sys/log/yagnats_og_client/yagnats_og_client.log'].map do |path|
     if File.exists? path
       File.read(path)
     else
       nil
     end
   end.compact.first
end

get '/truncate_logs/:lines' do
  lines = Integer(params[:lines])

  ['/var/vcap/sys/log/ruby_client/ruby_client.log',
   '/var/vcap/sys/log/yagnats_apcera_client/yagnats_apcera_client.log',
   '/var/vcap/sys/log/yagnats_og_client/yagnats_og_client.log'].map do |path|
     if File.exists? path
       `tail #{path} -#{lines} > #{path}`
       "truncated #{path} to #{lines} lines"
     else
       nil
     end
   end.compact.join("\n")
end
