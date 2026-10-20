# Harness 的 ACP 协议支持

## 配置

在自定义 `harnesses` 定义中设置 `protocol: acp`，即可连接任意兼容
Agent Client Protocol v1 的本机 agent 或 adapter。省略 `protocol` 或设置
`protocol: cli` 时沿用 CLI 调用方式。

```yaml
harnesses:
  coding-agent:
    protocol: acp
    binary: codex-acp
    # 可按 adapter 的启动方式设置 prefix_args。

steps:
  - id: review
    action: harness.run
    with:
      provider: coding-agent
      prompt: 检查当前目录的代码，概述主要模块，不修改文件。
    output: REVIEW
```

先安装 adapter，并按其文档完成认证。例如 Codex 的 ACP adapter 可以通过
`npm install -g @agentclientprotocol/codex-acp` 安装。其他 agent 可以使用各自的
ACP 模式或独立 adapter；Dagu 不要求固定的 agent 名称，也不自动安装 adapter。

`binary`、`prefix_args`、`flag_style` 和 `option_flags` 仍用于配置启动命令。
除了下述保留参数，`with` 中的其他参数仍转换为 adapter 的 CLI 启动选项。
提示词通过 ACP 发送，因此 ACP 定义不能包含 `prompt_mode`、`prompt_flag`、
`prompt_position`。`with.stdin` 会以空行连接到提示词末尾，不会写入协议管道。

## 行为约定

- 每次执行启动独立子进程，协商 ACP v1，创建新会话，然后发送一次提示。
  `session/new.cwd` 是步骤工作目录的绝对路径；进程继承步骤环境变量与资源限制。
- 使用 `github.com/coder/acp-go-sdk`，这是 ACP 官方文档收录的社区 Go SDK。
- `agent_message_chunk` 的文本按顺序拼接为步骤标准输出，可通过 `output`
  传递给后续步骤。思考、工具调用和协议 JSON 不进入该输出；agent 的 stderr
  保留在步骤错误日志中。
- 只有 `end_turn` 表示成功。协议错误、断连、拒答、长度限制、权限拒绝均使
  当前尝试失败，按现有 `fallback` 顺序继续；失败尝试的文本不进入成功输出。
- `with.acp_permission` 默认为 `deny`。只有显式设置 `allow` 才会选择 agent 提供的
  `allow_once` 选项；没有该选项时仍拒绝。权限策略仅控制 ACP 权限回调，不构成
  进程沙箱。不要把它当作对 agent 自有工具或文件访问的限制。
- 超时或停止时发送 `session/cancel`，随后终止进程树并回收管道、临时文件。
  取消执行不会触发 fallback。

## 当前边界

本版支持本机 stdio，不支持容器内 ACP、HTTP／WebSocket 传输、跨执行会话恢复、
交互式认证、MCP server 注入和交互式权限 UI。adapter 必须事先完成认证。
客户端不声明文件系统和终端回调能力；agent 可以使用自己的文件和命令工具。
依赖客户端提供文件或终端工具的 agent 需要后续扩展这些能力。

## 验收

- `internal/runtime/builtin/harness/acp_test.go` 使用独立 JSON-RPC 对端验证分块
  输出、权限、协商、错误、取消和 fallback。
- `conformance/spec067_harness/acp_test.go` 经真实 Dagu 二进制验证 YAML 加载、
  ACP 子进程、`with.stdin`、输出变量和下游步骤。
