require 'sinatra'

set :bind, '0.0.0.0'

def interface_name
  `ifconfig -s | cut -d " " -f 1`.chomp.split("\n").last
end

get '/' do
  "jello world, it's delicious #{`whoami`}"
end

get '/slowmo/:time' do
  cmd = `tc qdisc delete dev #{interface_name} root`
  cmd2 = `tc qdisc add dev #{interface_name} root netem delay #{params[:time]}`
  <<FOO
Time: #{Time.now}
Remove output: #{cmd}
Set: #{params[:time]} #{cmd2}
FOO
end

get '/dropmo/:rate' do
  out = `tc qdisc change dev eth0 root netem loss #{params[:rate]}%`
  <<FOO
Time: #{Time.now}
Reset out: #{out}
FOO
end

get '/reset' do
  out = `tc qdisc delete dev #{interface_name} root`
  <<FOO
Time: #{Time.now}
Reset out: #{out}
FOO
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

