package main

import (
	"flag"
	"fmt"
	"io/ioutil"
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
	PublishIntervalInSeconds int               `yaml:"publish_interval_in_seconds"`
	Name                     string            `yaml:"name"`
	PayloadSizeInBytes       int               `yaml:"payload_size_in_bytes"`
	NatsServers              []NatsInformation `yaml:"nats_servers"`
}

func main() {
	var configPath = flag.String("c", "/var/vcap/jobs/og_client/config/og_client.yml", "config path")
	flag.Parse()
	config := InitConfigFromFile(*configPath)

	c := &gosteno.Config{
		Sinks: []gosteno.Sink{
			gosteno.NewFileSink("/var/vcap/sys/log/yagnats_og_client/yagnats_og_client.log"),
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
		panic("Wrong auth or something.")
	}

	client.Subscribe(">", func(msg *yagnats.Message) {
		logger.Info(fmt.Sprintf("Got message: %s\n", msg.Payload))
	})

	count := 0
	for {
		publishMessage := []byte(fmt.Sprintf("publish_%s_%d", config.Name, count))
		publishWithReplyMessage := []byte(fmt.Sprintf("request_%s_%d", config.Name, count))

		publishMessage = padMessage(publishMessage, config.PayloadSizeInBytes)
		publishWithReplyMessage = padMessage(publishWithReplyMessage, config.PayloadSizeInBytes)

		client.Publish("yagnats.og.publish", publishMessage)
		logger.Info(fmt.Sprintf("Published message: %s\n", publishMessage))

		client.PublishWithReplyTo("yagnats.og.request", "yagnats.og.reply", publishWithReplyMessage)
		logger.Info(fmt.Sprintf("Requested message: %s\n", publishWithReplyMessage))

		count++
		time.Sleep(time.Duration(config.PublishIntervalInSeconds) * time.Second)
	}
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

func InitConfigFromFile(path string) *Config {
	var c *Config = new(Config)
	var e error

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
