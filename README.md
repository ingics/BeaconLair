# Ingics BeaconLair

Ingics BeaconLair is an IoT service architecture sample with Ingics BLE Gateway devices (iGS03) and BLE Sensor devices (iBS01/3/5 series). It can be used for testing, demonstration, and as a reference for your IoT application design.

## Requirement

The BeaconLair was designed to run services on Docker containers and managed by Docker Compose.
- Docker:  >18.06.0+
- Docker Compose (https://docs.docker.com/compose/)

Of course, at least one iGS03 gateway device and iBS0X sensor beacon are required.

## Quick Start

To run BeaconLair is just a single command.
```
docker-compose up -d --build
```

Check if all service start up.</br>
There should be 5 containers: <b>broker</b>, <b>grafana</b>, <b>influxdb</b>, <b>parser</b> and <b>rchost</b>.
```
$ sudo docker-compose ps
        Name                       Command               State                         Ports
---------------------------------------------------------------------------
beaconlair_broker_1     /docker-entrypoint.sh /usr ...   Up      0.0.0.0:1883->1883/tcp,:::1883->1883/tcp
beaconlair_grafana_1    /run.sh                          Up      0.0.0.0:3000->3000/tcp,:::3000->3000/tcp
beaconlair_influxdb_1   /entrypoint.sh influxd           Up      0.0.0.0:8086->8086/tcp,:::8086->8086/tcp
beaconlair_parser_1     /bin/sh -c ./parser              Up      0.0.0.0:9123->9000/tcp,:::9123->9000/tcp
beaconlair_rchost_1     /bin/sh -c ./rchost              Up      0.0.0.0:5000->5000/tcp,:::5000->5000/tcp, 5001/tcp
```

Then, configure the iGS03 connected to the network, and set up the remote control host & port to the BeaconLair host.
```
SYS CTRLHOST <Host IP>
SYS CTRLPORT 5000
REBOOT
```
The Host IP is how the iGS03 device connects to the BeaconLair Docker host. And the BeaconLair system will use it to configure the MQTT publish host, too.

To visit the BeaconLair HOME dashboard, open the URL "http://\<Host IP\>:3000/grafana/" in the browser.

![Home](../assets/beaconlair-home.png?raw=true)

Click on "Gateway Remote Control" link. You should see the iGS03 device.

![Remote Control](../assets/beaconlair-rc.png?raw=true)

Click on "Auto SetUp" button of the gateway, the remote control service will configure the gateway's data publish to the BeaconLair system. Go back to the HOME dashboard, and click on the link "Beacon List". If your iBS0X beacons are already powered on, you should see them listed on the dashboard with the latest sensor readings.

![Beacon List](../assets/beaconlair-beacons.png?raw=true)

Click on the sensor's reading area to the dashboard of the sensor reading chart.

![Sensor Readings](../assets/beaconlair-readings.png?raw=true)

## Architecture

![Block Diagram](../assets/beaconlair-block-diagram.jpeg?raw=true)

### MQTT Broker

BeaconLair use Eclipe Mosquitto as our MQTT broker service.
- [Eclipe Mosquitto](https://mosquitto.org/)
- [Eclipe Mosquitto Docker Image](https://hub.docker.com/_/eclipse-mosquitto)

### Database

BeaconLair use InfluxDB as backend database for storing the sensor readings and raw messages.
- [InfluxDB](https://www.influxdata.com/)
- [InfluxDB Docker Image](https://hub.docker.com/_/influxdb)

### Parser Service

The iBS0X sensor beacons broadcast the sensor readings in BLE advertising packets, and the iGS03 BLE gateway receives and transmits the raw data to the MQTT broker.

So we need a "Parser" service that subscribes to those raw data from the MQTT broker, parses them, and inserts the result into our backend database for further usage (display, alert system, analytic, ... ).

Beacon payload formats:
- [iBS01](https://www.ingics.com/doc/Beacon/BC0034_iBS_Sensor_Beacon_Payload.pdf)
- [iBS02](https://www.ingics.com/doc/Beacon/BC0034_iBS_Sensor_Beacon_Payload.pdf)
- [iBS03/04/05](https://www.ingics.com/doc/Beacon/BC0034_iBS_Sensor_Beacon_Payload.pdf)

In BeaconLair, this service was developed in Golang. We also provides parser libraries in some languages for customer to implement their-own service.

Parser libraries:
- [NodeJS](https://github.com/ingics/ingics-message-parser)
- [Golang](https://github.com/ingics/ingics-parser-go)
- [Python](https://github.com/ingics/ingics-message-parser-py)

### Gateway Remote Control Service

The iGS03 BLE gateway provides remote control capability. And the BeaconLair provides a simple remote control example (rchost) for:
- list or query device information
- auto-configuration for BeaconLair usage (data publish)
- device OTA upgrade
- RSSI filter modification

All telnet commands can be used in remote control connections.
- [telnet Commands](https://www.ingics.com/doc/Gateway/GW0017_iGS03_Telnet_Command.pdf)

### WebUI (Grafana)

The BeaconLair uses Grafana as frontend WebUI to provide dashboards for beacons, sensor readings, and gateway control.
- [Grafana](https://grafana.com/)
- [Grafana Docker image](https://hub.docker.com/r/grafana/grafana-enterprise)

## Configuration

There are four TCP port configurations placed in the .env file.
You can modify them if there is a port conflict issue on your docker host.
```
# MQTT port
MQTT_PORT=1883

# Gateway Remote Control port
REMOTE_CONTROL_PORT=5000

# InfluxDB HTTP port (Backend Database)
INFLUXDB_PORT=8086

# Grafana port (BeaconLair Dashboards)
GRAFANA_PORT=3000
```
After changing the port settings, restarting the containers is required.
```
docker-compose restart
```

## Trouble Shootings

#### Gateway not found in remote control dashboard

1. check <b>rchost</b> container status
   ```
   docker-compose ps rchost
   ```
2. check CTRL log on iGS03 telnet console
   ```
   > SYS SYSLOG
   [20220215 08:54:34] [MQTT_CLI/295/9] 0: Connected to <Host IP>:1883
   [20220215 08:54:34] [CTRL/345/9] 0: Connected: <Host IP>:5000
   [20220215 02:37:55] [NTP/394/2] 0: NTP time synced: 1644892675
   [19700101 00:00:02] [WIFI/949/1] 0: Joined WIFI-SSID/ch4
   ```
   You should see the "[CTRL/??/??]" entry, which means it connects to the remote control service successfully. If you did not see it, there may be a network issue. Please double-check the network settings.
3. check if any error in <b>rchost</b> logs
   ```
   docker-compose logs rchost
   ```

#### No beacons found in beacon list
1. check <b>parser</b> container status
   ```
   docker-compose ps parser
   ```
2. check logs of <b>parser</b>
   ```
   docker-compose logs -f parser
   parser_1    | 2022/02/15 08:08:01 [19]: readings,tag=iBS03T\ -\ 806FB087E146,stype=temperature value=21.00 1644912481243397983
   parser_1    | 2022/02/15 08:08:01 [19]: readings,tag=iBS03T\ -\ 806FB087E146,stype=humidity value=62 1644912481243397983
   ```
   If the parser service received the message from the broker (publish by iGS03), you should see logs like above.
