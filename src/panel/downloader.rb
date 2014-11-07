ARGV.map do |ip_of_nats_client|
  puts "--> downloading #{ip_of_nats_client}:4567/download_logs > logs/#{ip_of_nats_client}.log"
  `curl -L 'http://#{ip_of_nats_client}:4567/download_logs' > logs/#{ip_of_nats_client}.log`
end

puts "Done"

