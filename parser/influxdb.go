package main

import (
	"context"
	"fmt"
	"log"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/ingics/ingics-parser-go/igs"
)

// Build influx insert line for rawlog
func rawlogLine(msg *igs.Message) string {
	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	if msg.Timestamp() != nil {
		ts = fmt.Sprintf("%d", msg.Timestamp().UnixNano())
	}
	return fmt.Sprintf("rawlog,gw=%s,tag=%s rssi=%d,raw=\"%s\" %s\n",
		msg.Gateway(), msg.Beacon(), msg.RSSI(), msg.Payload(), ts)
}

// Build influx insert lines for sensors
func (adv AdvPacket) sensorLines() []string {
	var values []string
	var b2i = map[bool]int{false: 0, true: 1}
	model, _ := adv.packet.ProductModel()
	prefix := fmt.Sprintf("readings,tag=%s\\ -\\ %s", model, adv.msg.Beacon())
	ts := fmt.Sprintf("%d", time.Now().UnixNano())
	if adv.msg.Timestamp() != nil {
		ts = fmt.Sprintf("%d", adv.msg.Timestamp().UnixNano())
	}
	if v, ok := adv.packet.BatteryVoltage(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=battery value=%.2f %s", prefix, v, ts))
	}
	if v, ok := adv.packet.Temperature(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=temperature value=%.2f %s", prefix, v, ts))
	}
	if v, ok := adv.packet.TemperatureExt(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=temperature2 value=%.2f %s", prefix, v, ts))
	}
	if v, ok := adv.packet.Humidity(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=humidity value=%d %s", prefix, v, ts))
	}
	if v, ok := adv.packet.Range(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=range value=%d %s", prefix, v, ts))
	}
	if v, ok := adv.packet.CO2(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=co2 value=%d %s", prefix, v, ts))
	}
	if v, ok := adv.packet.GP(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=gp value=%.2f %s", prefix, v, ts))
	}
	if v, ok := adv.packet.Counter(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=counter value=%d %s", prefix, v, ts))
	}
	if v, ok := adv.packet.ButtonPressed(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=button value=%d %s", prefix, b2i[v], ts))
	}
	if v, ok := adv.packet.Moving(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=moving value=%d %s", prefix, b2i[v], ts))
	}
	if v, ok := adv.packet.HallDetected(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=hall value=%d %s", prefix, b2i[v], ts))
	}
	if v, ok := adv.packet.Falling(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=fall value=%d %s", prefix, b2i[v], ts))
	}
	if v, ok := adv.packet.PIRDetected(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=pir value=%d %s", prefix, b2i[v], ts))
	}
	if v, ok := adv.packet.IRDetected(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=ir value=%d %s", prefix, b2i[v], ts))
	}
	if v, ok := adv.packet.DinTriggered(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=din value=%d %s", prefix, b2i[v], ts))
	}
	if v, ok := adv.packet.Accel(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=accelx value=%d %s", prefix, v.X, ts))
		values = append(values,
			fmt.Sprintf("%s,stype=accely value=%d %s", prefix, v.Y, ts))
		values = append(values,
			fmt.Sprintf("%s,stype=accelz value=%d %s", prefix, v.Z, ts))
	}
	if v, ok := adv.packet.Accels(); ok {
		values = append(values,
			fmt.Sprintf("%s,stype=accelx value=%d %s", prefix, v[0].X, ts))
		values = append(values,
			fmt.Sprintf("%s,stype=accely value=%d %s", prefix, v[0].Y, ts))
		values = append(values,
			fmt.Sprintf("%s,stype=accelz value=%d %s", prefix, v[0].Z, ts))
	}
	return values
}

// Routine for writing packets by influx line protocol
func packetWriter() {
	rid := GoID()
	defer log.Printf("[%d] leave", rid)
	client := influxdb2.NewClient("http://influxdb:8086", "beaconlair")
	defer client.Close()
	for {
		writeAPI := client.WriteAPIBlocking("beaconlair", "db0")
		for p := range AdvPacketChannel {
			lines := p.sensorLines()
			for _, line := range lines {
				if err := writeAPI.WriteRecord(context.Background(), line); err != nil {
					log.Printf("[%d] %s\n", rid, err)
					break
				} else {
					log.Printf("[%d]: %s\n", rid, line)
				}
			}
			if err := writeAPI.WriteRecord(context.Background(), rawlogLine(p.msg)); err != nil {
				log.Printf("[%d] %s\n", rid, err)
				break
			}
		}
	}
}

func InitInfluxDBClient() {
	go packetWriter()
}
