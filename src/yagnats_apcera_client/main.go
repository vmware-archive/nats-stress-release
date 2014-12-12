package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime/debug"
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
	PublishIntervalInSeconds float64           `yaml:"publish_interval_in_seconds"`
	Name                     string            `yaml:"name"`
	PayloadSizeInBytes       int               `yaml:"payload_size_in_bytes"`
	NatsServers              []NatsInformation `yaml:"nats_servers"`
	Population               int               `yaml:"population"`
	APIKey                   string            `yaml:"datadog_api_key"`
}

var config Config

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}

		fmt.Fprintln(os.Stderr, debug.Stack())
	}()

	var configPath = flag.String("c", "/var/vcap/jobs/yagnats_apcera_client/config/yagnats_apcera_client.yml", "config path")
	flag.Parse()
	config = InitConfigFromFile(*configPath)

	fmt.Fprintln(os.Stderr, "DAN YOU CAUSED AN ERROR!!!")
	fmt.Println("DAN NO ERROR BOO YEAH")

	c := &gosteno.Config{
		Sinks: []gosteno.Sink{
			gosteno.NewFileSink("/var/vcap/sys/log/yagnats_apcera_client/yagnats_apcera_client.log"),
			//gosteno.NewFileSink("/tmp/yagnats_apcera_client.log"),
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

		fmt.Fprintln(os.Stderr, "DAN YOU MESSED UP")
		fmt.Fprintln(os.Stderr, debug.Stack())

		os.Exit(1)
	})

	client.Subscribe(">", func(msg *nats.Msg) {
		err := communicateMetric([]byte(fmt.Sprintf("received---%s", string(msg.Data))))
		if err != nil {
			logger.Error(err.Error())
		}
		// if matched, _ := regexp.Match("^publish--", msg.Data); matched {
		// 	publishMessage := []byte(fmt.Sprintf("received_publish--%s--%s", config.Name, msg.Data))
		// 	client.Publish("yagnats.apcera.publish", publishMessage)
		// }
	})

	count := 0

	for {
		publishMessage := []byte(fmt.Sprintf("publish--%s--%d--", config.Name, count))

		publishMessage = padMessage(publishMessage, config.PayloadSizeInBytes)
		errP := client.Publish("yagnats.apcera.publish", publishMessage)

		if errP != nil {
		   fmt.Fprintln(os.Stderr, "Error in publish")
		   fmt.Fprintln(os.Stderr, errP.Error())
		   // logger.Error(err.Error())
		}

		err := communicateMetric([]byte(fmt.Sprintf("sent---%s", string(publishMessage))))
		if err != nil {
		   logger.Error(err.Error())
		}


		count++
		time.Sleep(time.Duration(config.PublishIntervalInSeconds * float64(time.Second)))
	}
}

func communicateMetric(message []byte) error {
	resp, err := http.Post("http://127.0.0.1:4568/messages", "application/text", bytes.NewReader(message))
	resp.Body.Close()
	return err
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
