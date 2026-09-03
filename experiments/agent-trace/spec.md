# agent-trace：从最简实现观测 Agent 系统的完整链路

Status: needs-triage（初步方案，待评审）

## Problem Statement

`langfuse-otel` 实验证明了"如何把一个 Agent 的执行树观测到 Langfuse"，但其中的 Agent 只是固定两轮的纸模型：模型是硬编码状态机、工具没有注册表、消息没有真实流转。学习者看完它知道 Span 长什么样，却仍不知道一个 Agent 系统本身如何运转——决策循环如何驱动、消息如何在模型与工具之间往返、工具调用如何被分发和执行、错误如何退出。

本实验把视角从"观测载体"转向"观测对象"：在 `experiments/agent-trace/` 内用模块化方式实现一个最小的真实 Agent 系统，让学习者能观察它的一次完整链路：用户请求 → 模型决策 → 工具调用 → 结果回填 → 最终回答。保持仓库传统：可重复、零网络依赖、不依赖真实大模型、不产生费用。

## Solution

在 `experiments/agent-trace/` 目录内按职责拆分模块（不修改已完成的 `langfuse-otel`），实现由三个可替换组件构成的 ReAct Agent：

```text
experiments/agent-trace/
├── spec.md          # 本规格
├── main.go          # package main：组装组件并演示一次完整链路
├── agent/           # ReAct 决策循环（核心学习点）
│   ├── agent.go     #   Agent 类型：持有 LLM 与工具注册表，驱动多轮循环
│   └── agent_test.go
├── llm/             # 模型 seam（先以可编程 Fake 验证，真实接入待单独原型）
│   ├── message.go   #   Message 角色/内容模型（模型域对象）
│   ├── llm.go       #   LLM 接口：输入消息历史，输出一条模型消息
│   └── fake.go      #   确定性脚本 Fake，按消息历史返回 tool_call 或最终答案
├── tool/            # 工具 seam
│   ├── tool.go      #   Tool 接口 + Registry 注册/分发
│   └── knowledge.go #   示例工具：查询进程内静态知识源（首版无 DB/HTTP）
└── README.md        # 实验说明（后续实现阶段补齐）
```

一次完整链路：

```text
main.Run
└── agent.Agent.Run(prompt)
    ├── round 1: llm.Generate(messages)          → assistant: tool_call(knowledge_search)
    ├──          tool.Registry.Dispatch(tool_call) → knowledge 工具执行，返回结果
    ├──          回填 role=tool 消息到历史
    ├── round 2: llm.Generate(messages)          → assistant: 最终回答（结束）
    └── 返回最终消息与完整消息历史（可观测产物）
```

边界行为：最大轮数上限、工具不存在、工具执行错误、模型空响应——都结束循环并携带上下文返回错误。

## Module Decisions

- 目录内子包归属同一个根 module（`harukizmoe/go-exp`），互不导入其他实验目录；抽象保留在本实验内——当前只有这一个真实调用方，不提取共享包。
- seam 数量保持最小：`llm.LLM` 与 `tool.Tool` 两个接口，加 `tool.Registry` 一个分发点。Agent 只依赖接口，不依赖 Fake 实现；测试经接口注入确定性组件。
- `Message` 归口 `llm` 包：它是模型域的输入输出对象；`tool` 包只处理 `ToolCall`/`ToolResult`，由 `agent` 把工具结果转换并回填为 `role=tool` 消息。
- `agent.Run(ctx, prompt) (answer string, history []llm.Message, err error)` 是主要测试 seam，调用方传入 `context.Context`；取消/超时路径可验证。
- 知识工具首版查询进程内静态键值片段，与 `langfuse-otel` 的知识片段解耦，不引入数据库或 HTTP。

## Observability Decisions

- 默认观测产物：按时间序打印的链路记录 + 最终消息历史转储。学习者直接看到消息如何随轮次增长，这是 Agent 系统区别于普通函数的本质。
- 是否叠加 OpenTelemetry Span 树作为第二视角（复用 `langfuse-otel` 已验证的内存 exporter，不连 Langfuse），留待单独原型评估后再定，不在首版范围。
- 与 `langfuse-otel` 的分工：那里教"外部观测平台如何呈现执行树"，这里教"执行树本身为什么长这样"。

## User Stories（节选）

1. 作为学习者，我希望不配置任何凭证、不访问网络就运行一个 Agent，以便只关注系统本身。
2. 作为学习者，我希望 Agent 循环是真实的（消息历史随轮次增长、由模型输出决定下一步），以便理解 ReAct 决策循环。
3. 作为学习者，我希望模型请求工具时工具经注册表分发执行，以便看到"模型决策"与"工具执行"两个边界。
4. 作为学习者，我希望工具结果以 role=tool 消息回填并进入下一轮模型输入，以便观察 Agent 从工具回到模型的完整回路。
5. 作为学习者，我希望看到工具不存在、工具失败、超轮数、空响应等退出路径，以便理解 Agent 的失败语义。
6. 作为学习者，我希望每个模块（agent/llm/tool）有独立测试且通过接口注入确定性组件，以便理解 seam 与依赖倒置。
7. 作为维护者，我希望 Fake LLM 行为可编程（脚本驱动），以便用同一套接口验证正常、重试与错误路径。
8. 作为维护者，我希望本实验不修改 `langfuse-otel` 的任何文件，以便两个学习成果各自保持完整基线。

## Open Decisions（后续单独原型验证）

- LLM seam 的精确边界：消息格式、是否保留 tool_calls 原始结构、Fake 脚本的形态；原型将先验证这一契约再固化为接口。
- 真实 LLM 接入候选（本地网关或远端 API）与适配成本——原型后决策，本实验默认零网络。
- 是否把链路叠加 OTel Span 树打印。

## Testing Decisions

- 正常路径：多轮 tool_call → 回填 → 最终回答，验证答案与消息历史不变式（顺序、角色交替、工具结果被第二轮回填）。
- 错误路径：工具不存在、工具返回错误、超过最大轮数、模型返回空响应，验证返回错误且不泄漏 goroutine。
- Fake LLM 用脚本表驱动：每步按消息历史选择下一输出；测试直接构造脚本覆盖各分支。
- 并发/取消：`go test -race`；`Run` 接受 Context，超时路径有测试。
- 完成实现后运行：`go test -race ./experiments/agent-trace/...`、`go vet ./...`、`go run ./experiments/agent-trace`。

## Out of Scope

- 真实 LLM 接入（原型后另行决策）、多 Agent 协作、Agent 记忆持久化。
- 工具并行调用、流式输出、鉴权、生产队列与异步 Worker。
- 修改 `langfuse-otel` 或提取实验间共享包。
- 新增第三方依赖（复用根 module 已有的 `otel` 依赖即可，首版甚至不需要）。

## Next Steps

1. 评审本 spec 并创建 GitHub Issue 追踪（本文档链接进 Issue 正文）。
2. LLM seam 单独原型设计（决策 open decision 1）。
3. 依次实现 tool → agent → main 编排与演示。
4. 测试、README、验证命令收尾。
