# internal/pkg/dataformat

## 简介 (Chinese)

`dataformat` 包提供统一的点位原始字节数据格式化能力，支持在 Modbus、OPC UA、BACnet 等驱动中复用。通过单一入口 `FormatPoint`，可以将不同协议返回的原始字节解析为字符串形式，方便前端展示与规则引擎使用。

> 说明：本 README 中所有中文文案为初稿，仅用于评审与讨论，最终合并前请根据实际需求进行确认与修改。

### 支持的格式类型

- 有符号整型（8/16/32 位，大端/小端）
- 无符号整型（8/16/32 位，大端/小端）
- 十六进制字符串（带 `0x` 前缀，自动大写，长度按字节数补全）
- 二进制字符串（带 `0b` 前缀，按位宽补齐前导零）
- 32 位浮点 Long Float，支持 ABCD/CDAB/BADC/DCBA 四种字节顺序
- 64 位浮点 Double，支持 ABCD/CDAB/BADC/DCBA 四种 16 位字序组合
- 16 位自定义计算公式（表达式变量为 `v`，底层使用 16 位有符号整型）
- 32 位自定义计算公式（表达式变量为 `v`，底层使用 32 位有符号整型）

### 函数列表

核心入口：

```go
func FormatPoint(raw []byte, order binary.ByteOrder, fmtType FormatType, expr string) (string, error)
```

枚举类型：

```go
type FormatType int

const (
    FormatSignedInt FormatType = iota + 1
    FormatUnsignedInt
    FormatHexString
    FormatBinaryString
    FormatFloat32
    FormatFloat64
    FormatExpr16
    FormatExpr32
)
```

### 字节序与字序

- 对于整型类型：
  - `order` 使用标准 `binary.BigEndian` 或 `binary.LittleEndian` 表示字节序。
- 对于 4 字节/8 字节浮点：
  - 提供额外的字序实现：

```go
var (
    OrderABCD binary.ByteOrder
    OrderCDAB binary.ByteOrder
    OrderBADC binary.ByteOrder
    OrderDCBA binary.ByteOrder
)
```

这些常量同时实现 `binary.ByteOrder` 接口，并在内部包含 16 位寄存器级别的重排逻辑，用于常见的 Modbus 浮点 ABCD/CDAB/BADC/DCBA 顺序。

### 表达式语法

- 变量：
  - 仅允许 `v`，表示当前点位解析出的整数值。
- 常量：
  - 支持十进制整型常量，例如 `0`、`1`、`65535`。
- 运算符（按优先级从低到高）：
  - `|` 按位或
  - `^` 按位异或
  - `&` 按位与
  - `<<`、`>>` 左移 / 右移
  - `+`、`-` 加减
  - `*`、`/` 乘除
- 括号：
  - 支持 `(` 和 `)` 改变运算优先级。
- 安全限制：
  - 不支持函数调用、比较运算符、逻辑运算符等。
  - 解析阶段即对非法字符、未闭合括号、非法运算符做校验，返回错误而非 panic。
  - 运行阶段对加减乘除和移位进行溢出检测与除零检测，发生异常时返回错误。

## 使用示例 (Chinese)

### Modbus 读取 16 位寄存器并格式化为有符号整型

```go
raw := []byte{0x01, 0x00}
val, err := dataformat.FormatPoint(raw, binary.BigEndian, dataformat.FormatSignedInt, "")
// val == "256"
```

### Modbus 读取 32 位浮点，ABCD 字节顺序

```go
raw := []byte{0x42, 0xf6, 0xe9, 0x79}
val, err := dataformat.FormatPoint(raw, dataformat.OrderABCD, dataformat.FormatFloat32, "")
```

### BACnet 将 2 字节值格式化为十六进制和二进制

```go
raw := []byte{0xde, 0xad}
hexStr, _ := dataformat.FormatPoint(raw, binary.BigEndian, dataformat.FormatHexString, "")
binStr, _ := dataformat.FormatPoint(raw, binary.BigEndian, dataformat.FormatBinaryString, "")
```

### 自定义公式：按比例缩放并偏移

假设 16 位寄存器读出原始值 `v`，需要进行 `(v * 0.1) - 5` 的计算，考虑到表达式当前仅支持整数运算，可在配置层预先放大或缩小比例，再通过寄存器 `Scale/Offset` 组合使用，或在规则引擎中做浮点运算。当前版本的 `FormatExpr16/FormatExpr32` 更适合纯整数与位运算场景，如状态字解析、标志位组合等。

### 自定义公式：状态字解析示例

```go
raw := []byte{0x00, 0x0f} // 0000 0000 0000 1111
expr := "((v & 3) | 4) ^ 1"
val, err := dataformat.FormatPoint(raw, binary.BigEndian, dataformat.FormatExpr32, expr)
// val == "6"
```

## English Overview

The `dataformat` package provides a shared point data formatting utility for all southbound drivers (Modbus, OPC UA, BACnet). It converts raw bytes into human‑readable strings via a single entry point:

```go
func FormatPoint(raw []byte, order binary.ByteOrder, fmtType FormatType, expr string) (string, error)
```

### Supported formats

- Signed integers (8/16/32 bits, big‑endian and little‑endian)
- Unsigned integers (8/16/32 bits, big‑endian and little‑endian)
- Hex string (`0x` prefix, uppercase, padded to full byte width)
- Binary string (`0b` prefix, padded with leading zeros)
- 32‑bit float (long float) with ABCD/CDAB/BADC/DCBA register order
- 64‑bit float (double) with ABCD/CDAB/BADC/DCBA register order
- 16‑bit custom expression, using integer variable `v`
- 32‑bit custom expression, using integer variable `v`

Expressions support `+ - * / & | ^ << >>` and parentheses, with proper operator precedence. Only integer constants and variable `v` are allowed; there is no function call, no system I/O, and no network access.

### Error handling

- All errors are returned as Go `error` values; there is no panic in normal paths.
- The package wraps errors with:
  - operation (`decode` or `eval`)
  - target format type
  - byte order name
  - original byte slice (hex encoded)
- Error messages are in Chinese to match existing logging conventions in this project; function names and enums remain English.

### Safety and sandboxing

- The expression engine is a small custom AST parser implemented in pure Go.
- Only integer arithmetic and bitwise operations are supported.
- Division by zero, shift overflows, and integer overflows are detected and reported.
- No reflection, no dynamic imports, and no side‑effects beyond pure computation.

## Benchmarks

Benchmarks are located in `format_test.go`:

```bash
go test ./internal/pkg/dataformat -run . -bench . -benchmem
```

Sample result on a development machine:

- `BenchmarkFormatPoint_Int16` ≈ 17 ns/op
- `BenchmarkFormatPoint_Float32Expr` ≈ 190–230 ns/op

Since existing drivers have not yet been fully refactored to use `FormatPoint`, there is currently no measurable end‑to‑end performance regression on Modbus/BACnet/OPC UA paths. After integration, we can add driver‑level benchmarks (old vs new) and verify that the overhead stays within the required 5%.

## Integration status

- `internal/pkg/dataformat` provides the shared API and unit tests.
- `internal/driver/modbus/format_test.go`, `opcua/format_test.go`, and `bacnet/format_test.go` demonstrate how drivers import and call `FormatPoint`, ensuring build‑time integration without coupling driver logic into this package.

