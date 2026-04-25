# Examples

This directory contains the runnable examples for the cleaned-up Yggdrasil v3 layout.

## Structure

```text
examples/
├── proto/       # protobuf source files
├── protogen/    # generated Go code used by the examples
├── sample/      # smallest end-to-end sample
└── advanced/    # focused feature examples
```

## Recommended Path

1. Start with [`sample/`](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/sample).
2. Move to [`advanced/streaming/`](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/advanced/streaming).
3. Read [`advanced/rest/`](/Users/zhangwei/go/src/github.com/codesjoy/yggdrasil/examples/advanced/rest) if you need generated REST bindings.
4. Explore `config`, `metadata`, `error-handling`, and `load-balancing` next.

## Common Commands

```bash
cd examples/sample/server
go run main.go
```

```bash
cd examples/sample/client
go run main.go
```

## Notes

- `examples/go.mod` is the example module root.
- `examples/protogen` is a separate generated-code module used by imports such as `github.com/codesjoy/yggdrasil/v3/examples/protogen/...`.
- Advanced examples are intentionally small and focused on one capability at a time.
- **协议**: gRPC、HTTP/REST
- **配置**: YAML、环境变量、命令行
- **日志**: slog (Go 1.21+)
- **代码生成**: protoc、buf

## 扩展阅读

- [Yggdrasil 主文档](../../README.md)
- [Yggdrasil 中文文档](../../README_CN.md)
- [Protocol Buffers 文档](https://protobuf.dev/)
- [gRPC 文档](https://grpc.io/docs/)
- [REST API 设计指南](https://cloud.google.com/apis/design)

## 反馈与贡献

如果你在使用示例过程中遇到问题或有改进建议，欢迎：

1. 提交 [Issue](https://github.com/codesjoy/yggdrasil/issues)
2. 提交 [Pull Request](https://github.com/codesjoy/yggdrasil/pulls)
3. 在社区讨论

## 许可证

本示例遵循 Yggdrasil 项目的 [Apache License 2.0](../../LICENSE)。
