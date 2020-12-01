## Service

### Install

#### 1. copy binary file

```bash
cp mmp-go /usr/bin/
```

#### 2. add service file

```bash
# copy service file to systemd
cp systemd/mmp-go.service /etc/systemd/system/
```

#### 3. add config file: config.json

```bash
# copy config file to /etc/mmp-go/
mkdir /etc/mmp-go/
cp example.json /etc/mmp-go/config.json
```

#### 4. enable and start service: mmp-go

```bash
# enable and start service
systemctl enable --now mmp-go
```

See [mmp-go.service](mmp-go.service)