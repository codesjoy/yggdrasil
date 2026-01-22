# Multi SDK

该场景用于演示：通过 `sdk` 字段选择 Polaris SDKContext（以 name 区分），从而在同一个进程里使用多套 Polaris 配置。

示例配置片段：

```yaml
yggdrasil:
  polaris:
    blue:
      config_file: "./polaris-blue.yaml"
      token: "token-blue"
    green:
      addresses:
        - "127.0.0.1:8091"
      config_addresses:
        - "127.0.0.1:8093"
      token: "token-green"

  registry:
    schema: polaris
    config:
      sdk: blue
      namespace: "default"

  resolver:
    green:
      schema: polaris
      config:
        sdk: green
        namespace: "default"
```
