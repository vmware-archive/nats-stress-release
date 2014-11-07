

ARGV.drop(1).each do |ip_of_nats_client|
  puts "--> truncationg #{ip_of_nats_client}:4567/truncate_logs/#{ARGV[0]}"
  `curl -L 'http://#{ip_of_nats_client}:4567/truncate_logs/#{ARGV[0]}'`
end

puts "Done"

