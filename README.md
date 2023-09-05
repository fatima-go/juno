# juno #

juno process is ...

# application properties #

You can use juno properties like below.

name     | type   | default | remark
---------:|:-------|:--------| :-----
gateway.address  | string | 0.0.0.0 | jupiter ip address
gateway.port | int    | 9190    | jupiter listen port
webserver.address | string | 0.0.0.0 | juno listen ip address
webserver.port | int    | 9180    | juno listen port
remote.operation.allow | bool   | true    | remote operation(e.g roproc, rostop, ...) allow or not