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
remote.operation.allow | bool   | true    | Allow remote process registration/removal (`roproc`) and start/stop (`rostart`, `rostop`). Does not restrict deployment, queries, cron, or log-level operations; API authentication and roles still apply.
clip.binary.max.bytes | int | 1073741824 | Maximum clip binary download/upload size (capped at 1 GiB)
clip.binary.write.timeout.seconds | int | 0 | Write timeout for clip binary downloads (0 = unlimited)
clip.binary.upload.timeout.seconds | int | 0 | Overall timeout for clip binary uploads (0 = unlimited); a stalled upload is always cut off after 60s without data
