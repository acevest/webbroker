# WebBroker

这是一个用go语言实现支持多个web虚拟主机的broker

`config.yaml`的配置方法

`domain`是虚拟主机名
`host`是虚拟主机的实际地址

```
http:
  - domain: abc.com
    host: 127.0.0.1:81

  - domain: def.com
    host: 127.0.0.1:82


https:
  - domain: abc.com
    host: 127.0.0.1:90
    cert: path/cert.crt
    key: path/key.pem
```
