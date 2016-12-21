package main

import (
	"encoding/json"
	"fmt"
	"github.com/brutella/hc"
	"github.com/brutella/hc/accessory"
	"github.com/yosssi/gmq/mqtt"
	"github.com/yosssi/gmq/mqtt/client"
	"io/ioutil"
	"log"
	"os/user"
	"path/filepath"
)

func main() {

	usr, _ := user.Current()
	file, err := ioutil.ReadFile(filepath.Join(usr.HomeDir, ".config/hkdaemons/hkswitchd.json"))
	if err != nil {
		panic(err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(file, &config); err != nil {
		panic(err)
	}

	cli := client.New(&client.Options{
		// Define the processing of the error handler.
		ErrorHandler: func(err error) {
			log.Println(err)
		},
	})

	defer cli.Terminate()

	err = cli.Connect(&client.ConnectOptions{
		Network:  "tcp",
		Address:  config["broker"].(string),
		ClientID: []byte(config["client_id"].(string)),
	})
	if err != nil {
		panic(err)
	}

	var accessories []*accessory.Accessory

	configArr, ok := config["accessories"].([]interface{})
	if !ok {
		panic(err)
	}

	for _, accessory_config := range configArr {
		accessory_config := accessory_config.(map[string]interface{})
		info := accessory.Info{
			Name:         accessory_config["name"].(string),
			Manufacturer: accessory_config["manufacturer"].(string),
		}
		control_topic := fmt.Sprintf("switch_%s/control", accessory_config["machine_id"].(string))
		state_topic := fmt.Sprintf("switch_%s/state", accessory_config["machine_id"].(string))

		acc := accessory.NewSwitch(info)

		acc.Switch.On.OnValueRemoteUpdate(func(on bool) {
			value := 0
			if on {
				value = 1
			}
			log.Printf("Switching %s", value)
			err = cli.Publish(&client.PublishOptions{
				QoS:       mqtt.QoS0,
				TopicName: []byte(control_topic),
				Message:   []byte(fmt.Sprintf("%d", value)),
			})
			if err != nil {
				panic(err)
			}
		})

		err = cli.Subscribe(&client.SubscribeOptions{
			SubReqs: []*client.SubReq{
				&client.SubReq{
					TopicFilter: []byte(state_topic),
					QoS:         mqtt.QoS0,
					// Define the processing of the message handler.
					Handler: func(topicName, message []byte) {
						new_value := "1" == string(message)
						if acc.Switch.On.GetValue() != new_value {
							log.Printf("Value externally changed to %t\n", new_value)
							acc.Switch.On.SetValue(new_value)
						}
					},
				},
			},
		})
		if err != nil {
			panic(err)
		}
		accessories = append(accessories, acc.Accessory)
	}

	transport_config := hc.Config{Pin: config["pin"].(string), StoragePath: filepath.Join(usr.HomeDir, ".config/hkdaemons/data/hkswitchd")}
	t, err := hc.NewIPTransport(transport_config, accessories[0], accessories[1:]...)
	if err != nil {
		log.Fatal(err)
	}

	hc.OnTermination(func() {
		t.Stop()
	})

	t.Start()
}
