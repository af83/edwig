require 'fileutils'

$server = 'http://localhost:8081'

Before do
  unless File.directory?("tmp")
    FileUtils.mkdir_p("tmp")
  end
  unless File.directory?("log")
    FileUtils.mkdir_p("log")
  end
  system "EDWIG_ENV=test go run edwig.go -debug -pidfile=tmp/pid -testuuid -testclock=20170101-1200 api -listen=localhost:8081 >> log/edwig.log 2>&1 &"

  time_limit = Time.now + 30
  while
    sleep 0.5

    begin
      response = RestClient::Request.execute(method: :get, url: "#{$server}/_status", timeout: 1)
      break if response.code == 200 && response.body == '{ "status": "ok" }'
    rescue Exception # => e
      # puts e.inspect
    end

    raise "Timeout" if Time.now > time_limit
  end
end

After do
  pid = IO.read("tmp/pid")
  Process.kill('KILL',pid.to_i)
end