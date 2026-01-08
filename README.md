# learn-pub-sub-starter (Peril)

This is the starter code used in Boot.dev's [Learn Pub/Sub](https://learn.boot.dev/learn-pub-sub) course.

## Podman
To start rabbitmq on Fedora 43:

`podman run -it --rm --name rabbitmq --network pasta:--ipv4-only -p 5672:5672 -p 15672:15672 rabbitmq:3.13-management`

The `--network pasta:--ipv4-only` flag disables IPv6, which was causing connection reset by peer errors on rootless podman when trying to connect to localhost.
https://github.com/containers/podman/issues/25674
