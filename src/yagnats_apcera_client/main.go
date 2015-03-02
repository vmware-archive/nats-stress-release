package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry-incubator/candiedyaml"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/yagnats"
)

type NatsInformation struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
}

type Config struct {
	PublishIntervalInSeconds string            `yaml:"publish_interval_in_seconds"`
	Name                     string            `yaml:"name"`
	PayloadSizeInBytes       int               `yaml:"payload_size_in_bytes"`
	NatsServers              []NatsInformation `yaml:"nats_servers"`
	Population               int               `yaml:"population"`
	APIKey                   string            `yaml:"datadog_api_key"`
}

var config Config
var sendIntervalCount uint64 = 0
var recvIntervalCount uint64 = 0
var count int = 0

func main() {
	var configPath = flag.String("c", "/var/vcap/jobs/yagnats_apcera_client/config/yagnats_apcera_client.yml", "config path")
	flag.Parse()
	config = InitConfigFromFile(*configPath)

	c := &gosteno.Config{
		Sinks: []gosteno.Sink{
			gosteno.NewFileSink("/var/vcap/sys/log/yagnats_apcera_client/yagnats_apcera_client.log"),
		},
		Level:     gosteno.LOG_INFO,
		Codec:     gosteno.NewJsonCodec(),
		EnableLOC: true,
	}
	gosteno.Init(c)
	logger := gosteno.NewLogger(config.Name)

	urls := make([]string, len(config.NatsServers))
	for _, server := range config.NatsServers {
		urls = append(urls, fmt.Sprintf("nats://%s:%s@%s:%d", server.Username, server.Password, server.Host, server.Port))
	}

	client, err := yagnats.Connect(urls)
	if err != nil {
		logger.Error(err.Error())
		panic(err.Error())
	}

	client.AddReconnectedCB(func(conn *nats.Conn) {
		fmt.Sprintf("NATS Client Reconnected. Server URL: %s\n", conn.Opts.Url)
		logger.Info(fmt.Sprintf("NATS Client Reconnected. Server URL: %s", conn.Opts.Url))
	})

	client.AddClosedCB(func(conn *nats.Conn) {
		err := errors.New(fmt.Sprintf("NATS Client Closed. nats.Conn: %+v", conn))
		logger.Error(fmt.Sprintf("NATS Closed: %s", err.Error()))
		os.Exit(1)
	})

	client.Subscribe(">", func(msg *nats.Msg) {
		atomic.AddUint64(&recvIntervalCount, 1)
		// if matched, _ := regexp.Match("^publish--", msg.Data); matched {
		// 	publishMessage := []byte(fmt.Sprintf("received_publish--%s--%s", config.Name, msg.Data))
		// 	client.Publish("yagnats.apcera.publish", publishMessage)
		// }
	})
	fmt.Fprintln(os.Stderr, "Send")
	tickSend := time.NewTicker(processDuration(config.PublishIntervalInSeconds + "s"))
	fmt.Fprintln(os.Stderr, "Metric")
	tickMetric := time.NewTicker(processDuration("0.333s"))

	for {
		select {
		case <-tickSend.C:
			// fmt.Fprintln(os.Stderr, "--> Send")
			publishMessage := []byte(fmt.Sprintf("publish--%s--%d--", config.Name, count))

			publishMessage = padMessage(publishMessage, config.PayloadSizeInBytes)
			errP := client.Publish("yagnats.apcera.publish", publishMessage)
			atomic.AddUint64(&sendIntervalCount, 1)
			if errP != nil {
				logger.Error(err.Error())
			}
		case <-tickMetric.C:
			// fmt.Fprintln(os.Stderr, "--> Metric")
			recvLocal := atomic.SwapUint64(&recvIntervalCount, 0)
			sendLocal := atomic.SwapUint64(&sendIntervalCount, 0)
			body := strconv.FormatUint(recvLocal, 10) + ", " + strconv.FormatUint(sendLocal, 10)
			logger.Info(fmt.Sprintf("Metrics %s", body))
			resp, err := http.Post("http://127.0.0.1:4568/messages-new", "application/text", strings.NewReader(body))
			resp.Body.Close()
			if err != nil {
				logger.Error(err.Error())
			}
		}
	}
}

func processDuration(input string) time.Duration {
	metricDuration, err := time.ParseDuration(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Duration Fault", err.Error())
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "Duration", metricDuration)
	return metricDuration
}

func padMessage(message []byte, paddingLength int) []byte {
	if len(message) < paddingLength {
		a := make([]byte, paddingLength)
		for i := range a {
			a[i] = []byte(".")[0]
		}
		copy(a, message)
		return a
	}
	return message
}

func InitConfigFromFile(path string) Config {
	var e error
	c := Config{}

	b, e := ioutil.ReadFile(path)
	if e != nil {
		panic(e.Error())
	}

	e = c.Initialize(b)
	if e != nil {
		panic(e.Error())
	}

	return c
}

func (c *Config) Initialize(configYAML []byte) error {
	return candiedyaml.Unmarshal(configYAML, &c)
}
