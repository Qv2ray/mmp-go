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

### Auto-Reload
#### 1. enable and start
```bash
systemctl enable --now mmp-go-reload.timer
```

#### 2. customize
Execute the following command:
```bash
systemctl edit mmp-go-reload.timer
```

Fill in your customized values, for example:
```
# empty value means to remove the preset value
[Unit]
Description=
Description=Eight-Hourly Reload mmp-go service 

[Timer]
OnActiveSec=
OnActiveSec=8h
OnUnitActiveSec=
OnUnitActiveSec=8h
```

optionally do a daemon-reload afterwards:
```bash
systemctl daemon-reload
```
