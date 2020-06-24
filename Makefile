user = pi
srcdir = ~/i2vnc/deploy

# apt = sudo apt update && sudo apt-get install golang git xorg xserver-xorg-video-dummy -y
# clone = git clone https://github.com/runz0rd/i2vnc.git $(srcdir) && cd $(srcdir)
# build = sudo go build -o /usr/local/bin/i2vnc cmd/main.go && sudo chmod -x /usr/local/bin/i2vnc
build = env GOOS=linux GOARCH=arm GOARM=7 go build -o deploy/i2vnc cmd/main.go
scp = scp -r deploy/ $(user)@$(host):$(srcdir)
cdsrc = cd $(srcdir)
apt = sudo apt update && sudo apt-get install xorg xserver-xorg-video-dummy -y
bin = sudo cp i2vnc /usr/local/bin/i2vnc
config = sudo mkdir -p /usr/share/i2vnc/ && sudo cp config.yaml /usr/share/i2vnc/config.yaml
xorg = sudo cp xorg.conf /usr/share/X11/xorg.conf.d/xorg.conf
service = sudo cp i2vnc.service /lib/systemd/system/i2vnc.service && sudo chmod 644 /lib/systemd/system/i2vnc.service
systemctl =	sudo systemctl daemon-reload && sudo systemctl enable i2vnc.service && sudo systemctl start i2vnc.service

disable = sudo systemctl disable i2vnc.service && sudo systemctl daemon-reload
clean = sudo rm -rf $(srcdir) /usr/share/X11/xorg.conf.d/xorg.conf /lib/systemd/system/i2vnc.service /usr/local/bin/i2vnc /usr/share/i2vnc/

build.rpi:
	env GOOS=linux GOARCH=arm GOARM=7 go build -o deploy/i2vnc cmd/main.go

copy.rpi:
	scp -r deploy/ $(user)@$(host):$(srcdir)

deploy.rpi: build.rpi copy.rpi
	ssh $(user)@$(host) "$(cdsrc) && $(apt) && $(bin) && $(config) && $(xorg) && $(service) && $(systemctl)"

clean.rpi:
	ssh $(user)@$(host) "$(disable) && $(clean)"

