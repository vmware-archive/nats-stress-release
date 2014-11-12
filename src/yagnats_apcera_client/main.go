package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"
	"sync"
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

func dedup(xs *[]string) {
	found := make(map[string]bool)
	j := 0
	for i, x := range *xs {
		if !found[x] {
			found[x] = true
			(*xs)[j] = (*xs)[i]
			j++
		}
	}
	*xs = (*xs)[:j]
}

func main() {
	var configPath = flag.String("c", "/var/vcap/jobs/yagnats_apcera_client/config/yagnats_apcera_client.yml", "config path")
	flag.Parse()
	config := InitConfigFromFile(*configPath)

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

	messageTally := map[string][]string{}
	population := config.Population
	msgsCompleted := 0
	m := sync.Mutex{}
	client.Subscribe(">", func(msg *nats.Msg) {
		if matched, _ := regexp.Match("^publish--", msg.Data); matched {
			publishMessage := []byte(fmt.Sprintf("received_publish--%s--%s", config.Name, msg.Data))
			client.Publish("yagnats.apcera.publish", publishMessage)
		}

		r := regexp.MustCompile(fmt.Sprintf("^received_publish--.*?--publish--%s", config.Name))
		if r.Match(msg.Data) {
			m.Lock()
			r := regexp.MustCompile("^received_publish--(.*?)--(.*)")
			matches := r.FindSubmatch(msg.Data)
			originalMsg := string(matches[2])
			recepient := string(matches[1])
			if _, found := messageTally[originalMsg]; !found {
				messageTally[originalMsg] = []string{}
			}
			messageTally[originalMsg] = append(messageTally[originalMsg], recepient)
			stuff := messageTally[originalMsg]
			dedup(&stuff)
			messageTally[originalMsg] = stuff
			if len(messageTally[originalMsg]) == population {
				logger.Info(fmt.Sprintf("tallied up %s\n", strings.Join(messageTally[originalMsg], ",")))
				msgsCompleted++
				delete(messageTally, originalMsg)
			}
			m.Unlock()
		}
	})

	count := 0
	for {
		publishMessage := []byte(fmt.Sprintf("publish--%s--%d--", config.Name, count))
		//publishRequestMessage := []byte(fmt.Sprintf("request--%s--%d--", config.Name, count))

		publishMessage = padMessage(publishMessage, config.PayloadSizeInBytes)
		//publishRequestMessage = padMessage(publishRequestMessage, config.PayloadSizeInBytes)

		logger.Info(fmt.Sprintf("publishing %s\n", publishMessage))
		logger.Info(fmt.Sprintf("completed %d messages. outstanding: %d", msgsCompleted, len(messageTally)))
		datadog(config, "msgs.sent", count)
		datadog(config, "msgs.completed", msgsCompleted)
		datadog(config, "msgs.outstanding", len(messageTally))
		client.Publish("yagnats.apcera.publish", publishMessage)

		//logger.Info(fmt.Sprintf("requesting %s\n", publishRequestMessage))
		//client.PublishRequest("yagnats.apcera.request", "yagnats.apcera.reply", publishRequestMessage)

		count++
		time.Sleep(time.Duration(config.PublishIntervalInSeconds * float64(time.Second)))
	}
}

func datadog(config Config, metric string, value interface{}) {
	curl := `
		curl -s -X POST -H "Content-type: application/json" \
		-d '{ "series" :
						 [{"metric":"nats-stress.` + metric + `",
							"points":[[` + fmt.Sprintf("%d", time.Now().UTC().Unix()) + `, ` + fmt.Sprintf("%v", value) + `]],
							"type":"gauge",
							"host":"` + config.Name + `",
							"tags":["client:yagnats-apcera"]}
						]
				}' \
		'https://app.datadoghq.com/api/v1/series?api_key=` + config.APIKey + `'
	`
	cmd := exec.Command("bash", "-c", curl)
	cmd.Run()
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
