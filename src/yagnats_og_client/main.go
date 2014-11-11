package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
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

	received := map[string]int{}
	m := sync.Mutex{}
	client.Subscribe(">", func(msg *yagnats.Message) {
		messageParts := strings.Split(string(msg.Payload), "--")
			if len(messageParts) < 2 {
				return
			}
		messageType := messageParts[0]
		messageName := messageParts[1]
		if messageType == "publish" {
			m.Lock()
			if _, found := received[messageName]; !found {
				received[messageName] = 0
			}
			received[messageName] += 1
			m.Unlock()
			output := []string{}
			for k, v := range received {
				output = append(output, fmt.Sprintf("%s: %d", k, v))
			}
			logger.Info(strings.Join(output, " "))
		}
		// logger.Info(fmt.Sprintf("receiving %s\n", msg.Payload))
	})

	count := 0
	for {
		publishMessage := []byte(fmt.Sprintf("publish--%s--%d--", config.Name, count))
		publishWithReplyMessage := []byte(fmt.Sprintf("request--%s--%d--", config.Name, count))

		publishMessage = padMessage(publishMessage, config.PayloadSizeInBytes)
		publishWithReplyMessage = padMessage(publishWithReplyMessage, config.PayloadSizeInBytes)

		logger.Info(fmt.Sprintf("publishing %s\n", publishMessage))
		client.Publish("yagnats.og.publish", publishMessage)

		logger.Info(fmt.Sprintf("requesting %s\n", publishWithReplyMessage))
		client.PublishWithReplyTo("yagnats.og.request", "yagnats.og.reply", publishWithReplyMessage)

		count++
		time.Sleep(time.Duration(config.PublishIntervalInSeconds * float64(time.Second)))
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
