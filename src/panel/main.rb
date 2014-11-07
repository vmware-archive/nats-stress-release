require 'sinatra'

set :bind, '0.0.0.0'

get '/' do
  "jello world, it's delicious #{`whoami`}"
end

get '/ifconfig' do
  `ifconfig`
end

get '/slowmo/:interface/:time' do
  cmd = `tc qdisc delete dev #{params[:interface]} root`
  cmd2 = `tc qdisc add dev #{params[:interface]} root netem delay #{params[:time]}`
  <<FOO
Time: #{Time.now}
Remove output: #{cmd}
Set: #{params[:time]} #{cmd2}
FOO
end

get '/dropmo/:rate' do
  out = `tc qdisc change dev #{params[:interface]} root netem loss #{params[:rate]}%`
  <<FOO
Time: #{Time.now}
Reset out: #{out}
FOO
end

get '/reset/:interface' do
  out = `tc qdisc delete dev #{params[:interface]} root`
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
