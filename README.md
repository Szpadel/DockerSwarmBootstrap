To bootstrap it automatically use cloud-config like:

```coreos:
  units:
    - name: etcd2.service
      command: start
    - name: docker.service
      drop-ins:
        - name: "20-swarm.conf"
          content: |
            [Service]
            Environment="DOCKER_OPTS=-H=0.0.0.0:2376 -H unix:///var/run/docker.sock"
    - name: swarm-bootstrap.service
      command: start
      content: |
        [Unit]
        Description=Docker Swarm Bootstrap
        After=etcd2.service
        After=docker.service

        [Service]
        Type=oneshot
        ExecStartPre=/usr/bin/docker pull szpadel/docker-swarm-bootstrap
        ExecStart=/usr/bin/docker run --rm --network host -e DOCKER_API_VERSION=1.24 -e DOCKER_HOST=tcp://127.0.0.1:2376 szpadel/docker-swarm-bootstrap -size $managersNodesNumber -advert-ip $ip:2376

        [Install]
        WantedBy=multi-user.target
 etcd2:
    discovery: $discovery
    advertise-client-urls: "http://$ip:2379"
    initial-advertise-peer-urls: "http://$ip:2380"
    # listen on both the official ports and the legacy ports
    # legacy ports can be omitted if your application doesn't depend on them
    listen-client-urls: "http://$ip:2379,http://$ip:4001"
    listen-peer-urls: "http://$ip:2380,http://$ip:7001"```
