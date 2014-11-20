package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

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
	var configPath = flag.String("c", "/var/vcap/jobs/og_client/config/og_client.yml", "config path")
	flag.Parse()
	config = InitConfigFromFile(*configPath)

	c := &gosteno.Config{
		Sinks: []gosteno.Sink{
//			gosteno.NewFileSink("/var/vcap/sys/log/yagnats_og_client/yagnats_og_client.log"),
			gosteno.NewFileSink("/tmp/yagnats_og_client.log"),
		},
		Level:     gosteno.LOG_INFO,
		Codec:     gosteno.NewJsonCodec(),
		EnableLOC: true,
	}
	gosteno.Init(c)
	logger := gosteno.NewLogger(config.Name)

	connectionCluster := &yagnats.ConnectionCluster{}
	for _, server := range config.NatsServers {
		connectionCluster.Members = append(connectionCluster.Members, &yagnats.ConnectionInfo{
			Addr:     fmt.Sprintf("%s:%d", server.Host, server.Port),
			Username: server.Username,
			Password: server.Password,
		})
	}

	client := yagnats.NewClient()
	err := client.Connect(connectionCluster)
	if err != nil {
		logger.Error(err.Error())
		panic(err.Error())
	}

	client.Subscribe(">", func(msg *yagnats.Message) {
		err := communicateMetric([]byte(fmt.Sprintf("received---%s", string(msg.Payload))))
		if err != nil {
			logger.Error(err.Error())
		}
		if matched, _ := regexp.Match("^publish--", msg.Payload); matched {
			publishMessage := []byte(fmt.Sprintf("received_publish--%s--%s", config.Name, msg.Payload))
			client.Publish("yagnats.og.publish", publishMessage)
		}
	})

	count := 0

	for {
		publishMessage := []byte(fmt.Sprintf("publish--%s--%d--", config.Name, count))
		publishMessage = padMessage(publishMessage, config.PayloadSizeInBytes)

		client.Publish("yagnats.og.publish", publishMessage)
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
