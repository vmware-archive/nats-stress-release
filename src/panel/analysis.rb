require 'json'
require 'time'

BUCKET_SIZE = (ENV["SIZE"] || 1*60).to_i
#RADIUS = (ENV["RADIUS"] || 10).to_i

buckets = []
ref_time = Time.now.to_f
real_now = Time.now.to_f

class Bucket < Struct.new(:minute)
  def tally
    @tally ||= {}
  end
end

oldest_message_time = Time.now.to_f
ARGV.each do |client_log_file|
  File.read(client_log_file).each_line do |line|
    hsh = begin
            JSON.parse(line)
          rescue
            next
          end
    time = Time.at(hsh["timestamp"]).to_f
    #next if ref_time - time > 30*60
    oldest_message_time = time if time < oldest_message_time
    message = hsh["message"].chomp
    bucket = buckets.detect{|b| b.tally[message] }
    bucket ||= buckets.detect{|b| b.minute == time.to_i / 60 }
    if bucket.nil?
      bucket = Bucket.new(time.to_i / 60)
      buckets << bucket
    end
    if message =~ /receiving publish./
      bucket.tally[message] ||= 0
      bucket.tally[message] += 1
    end
  end
end

buckets.sort_by(&:minute).each do |b|
  failed = {}
  b.tally.values.each do |v|
    if v != ARGV.size
      puts "message #{k} was received by only #{v} clients, not #{ARGV.size}" if ENV["DEBUG"]
      failed[v] ||= 0
      failed[v] += 1
    end
  end

  puts "#{Time.at(b.minute*60).strftime("%H:%M:%S")} >>> #{Time.at(b.minute*60).strftime("%H:%M:%S")}--> #{b.tally.size} messages, #{failed} failed"
end
puts "took #{Time.now.to_f - real_now} seconds"
