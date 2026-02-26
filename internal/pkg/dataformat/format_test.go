package dataformat

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestFormatPoint_SignedInt(t *testing.T) {
	tests := []struct {
		name   string
		raw    []byte
		order  binary.ByteOrder
		expect string
	}{
		{
			name:   "int8 positive",
			raw:    []byte{0x7f},
			order:  binary.BigEndian,
			expect: "127",
		},
		{
			name:   "int8 negative",
			raw:    []byte{0x80},
			order:  binary.BigEndian,
			expect: "-128",
		},
		{
			name:   "int16 big endian",
			raw:    []byte{0x01, 0x00},
			order:  binary.BigEndian,
			expect: "256",
		},
		{
			name:   "int16 little endian",
			raw:    []byte{0x00, 0x01},
			order:  binary.LittleEndian,
			expect: "256",
		},
		{
			name:   "int32 negative",
			raw:    []byte{0xff, 0xff, 0xff, 0xfe},
			order:  binary.BigEndian,
			expect: "-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatPoint(tt.raw, tt.order, FormatSignedInt, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expect {
				t.Fatalf("expect %s, got %s", tt.expect, got)
			}
		})
	}
}

func TestFormatPoint_UnsignedInt(t *testing.T) {
	tests := []struct {
		name   string
		raw    []byte
		order  binary.ByteOrder
		expect string
	}{
		{
			name:   "uint8",
			raw:    []byte{0xff},
			order:  binary.BigEndian,
			expect: "255",
		},
		{
			name:   "uint16",
			raw:    []byte{0x12, 0x34},
			order:  binary.BigEndian,
			expect: "4660",
		},
		{
			name:   "uint32 little endian",
			raw:    []byte{0x01, 0x00, 0x00, 0x00},
			order:  binary.LittleEndian,
			expect: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatPoint(tt.raw, tt.order, FormatUnsignedInt, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expect {
				t.Fatalf("expect %s, got %s", tt.expect, got)
			}
		})
	}
}

func TestFormatPoint_HexAndBinary(t *testing.T) {
	raw := []byte{0x12, 0x34}
	hexStr, err := FormatPoint(raw, binary.BigEndian, FormatHexString, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hexStr != "0x1234" {
		t.Fatalf("expect 0x1234, got %s", hexStr)
	}

	binStr, err := FormatPoint(raw, binary.BigEndian, FormatBinaryString, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if binStr != "0b0001001000110100" {
		t.Fatalf("unexpected binary: %s", binStr)
	}
}

func TestFormatPoint_Float32_WordOrders(t *testing.T) {
	f := float32(123.456)
	bits := math.Float32bits(f)
	var base [4]byte
	binary.BigEndian.PutUint32(base[:], bits)

	rawABCD := []byte{base[0], base[1], base[2], base[3]}
	rawCDAB := []byte{base[2], base[3], base[0], base[1]}
	rawBADC := []byte{base[1], base[0], base[3], base[2]}
	rawDCBA := []byte{base[3], base[2], base[1], base[0]}

	tests := []struct {
		name  string
		raw   []byte
		order binary.ByteOrder
	}{
		{"ABCD", rawABCD, OrderABCD},
		{"CDAB", rawCDAB, OrderCDAB},
		{"BADC", rawBADC, OrderBADC},
		{"DCBA", rawDCBA, OrderDCBA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatPoint(tt.raw, tt.order, FormatFloat32, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			val, err := strconv.ParseFloat(got, 32)
			if err != nil {
				t.Fatalf("parse float failed: %v", err)
			}
			if diff := math.Abs(float64(f) - val); diff > 1e-3 {
				t.Fatalf("expect %v, got %v, diff %v", f, val, diff)
			}
		})
	}
}

func TestFormatPoint_Float64_WordOrders(t *testing.T) {
	f := 123.456789
	bits := math.Float64bits(f)
	var base [8]byte
	binary.BigEndian.PutUint64(base[:], bits)
	rawABCD := append([]byte{}, base[:]...)

	rawCDAB := make([]byte, 8)
	copy(rawCDAB[0:2], base[2:4])
	copy(rawCDAB[2:4], base[0:2])
	copy(rawCDAB[4:6], base[6:8])
	copy(rawCDAB[6:8], base[4:6])

	rawBADC := make([]byte, 8)
	rawBADC[0], rawBADC[1] = base[1], base[0]
	rawBADC[2], rawBADC[3] = base[3], base[2]
	rawBADC[4], rawBADC[5] = base[5], base[4]
	rawBADC[6], rawBADC[7] = base[7], base[6]

	rawDCBA := make([]byte, 8)
	copy(rawDCBA[0:2], base[6:8])
	copy(rawDCBA[2:4], base[4:6])
	copy(rawDCBA[4:6], base[2:4])
	copy(rawDCBA[6:8], base[0:2])

	tests := []struct {
		name  string
		raw   []byte
		order binary.ByteOrder
	}{
		{"ABCD", rawABCD, OrderABCD},
		{"CDAB", rawCDAB, OrderCDAB},
		{"BADC", rawBADC, OrderBADC},
		{"DCBA", rawDCBA, OrderDCBA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatPoint(tt.raw, tt.order, FormatFloat64, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			val, err := strconv.ParseFloat(got, 64)
			if err != nil {
				t.Fatalf("parse float failed: %v", err)
			}
			if diff := math.Abs(f - val); diff > 1e-6 {
				t.Fatalf("expect %v, got %v, diff %v", f, val, diff)
			}
		})
	}
}

func TestFormatPoint_Expression16(t *testing.T) {
	raw := []byte{0x00, 0x10}
	got, err := FormatPoint(raw, binary.BigEndian, FormatExpr16, "v*2+1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "33" {
		t.Fatalf("expect 33, got %s", got)
	}
}

func TestFormatPoint_Expression32(t *testing.T) {
	raw := []byte{0x00, 0x00, 0x01, 0x00}
	got, err := FormatPoint(raw, binary.BigEndian, FormatExpr32, "v>>2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "64" {
		t.Fatalf("expect 64, got %s", got)
	}
}

func TestFormatPoint_ExpressionError(t *testing.T) {
	raw := []byte{0x00, 0x10}
	_, err := FormatPoint(raw, binary.BigEndian, FormatExpr16, "v/0")
	if err == nil {
		t.Fatalf("expect error for divide by zero")
	}

	_, err = FormatPoint(raw, binary.BigEndian, FormatExpr16, "invalid(")
	if err == nil {
		t.Fatalf("expect parse error for invalid expression")
	}
}

func TestFormatPoint_ExpressionBitwiseAndShift(t *testing.T) {
	raw := []byte{0x00, 0x00, 0x00, 0x0f}
	got, err := FormatPoint(raw, binary.BigEndian, FormatExpr32, "((v & 3) | 4) ^ 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "6" {
		t.Fatalf("expect 6, got %s", got)
	}

	_, err = FormatPoint(raw, binary.BigEndian, FormatExpr16, "v<<64")
	if err == nil {
		t.Fatalf("expect error for shift overflow")
	}
}

func TestFormatPoint_ExpressionOverflow(t *testing.T) {
	raw := []byte{0x00, 0x01}
	_, err := FormatPoint(raw, binary.BigEndian, FormatExpr32, "9223372036854775807+1")
	if err == nil {
		t.Fatalf("expect overflow error")
	}
}

func TestFormatPoint_IntLengthErrors(t *testing.T) {
	_, err := FormatPoint([]byte{0x01, 0x02, 0x03}, binary.BigEndian, FormatSignedInt, "")
	if err == nil {
		t.Fatalf("expect error for invalid signed int length")
	}
	_, err = FormatPoint([]byte{0x01, 0x02, 0x03}, binary.BigEndian, FormatUnsignedInt, "")
	if err == nil {
		t.Fatalf("expect error for invalid unsigned int length")
	}
}

func TestFormatPoint_Errors(t *testing.T) {
	_, err := FormatPoint(nil, binary.BigEndian, FormatSignedInt, "")
	if err == nil {
		t.Fatalf("expect error for empty raw")
	}

	_, err = FormatPoint([]byte{0x01}, binary.BigEndian, FormatFloat32, "")
	if err == nil {
		t.Fatalf("expect error for invalid length float32")
	}

	_, err = FormatPoint([]byte{0x01}, binary.BigEndian, FormatFloat64, "")
	if err == nil {
		t.Fatalf("expect error for invalid length float64")
	}

	_, err = FormatPoint([]byte{0x01}, binary.BigEndian, 99, "")
	if err == nil {
		t.Fatalf("expect error for invalid format type")
	}
}

func TestCustomOrder_WordAndName(t *testing.T) {
	o := customOrder{
		n4: [4]int{2, 3, 0, 1},
		n8: [8]int{6, 7, 4, 5, 2, 3, 0, 1},
		id: "TEST",
	}

	b2 := []byte{0x11, 0x22}
	if got := o.Uint16(b2); got != binary.BigEndian.Uint16(b2) {
		t.Fatalf("Uint16 mismatch")
	}
	var out2 [2]byte
	o.PutUint16(out2[:], o.Uint16(b2))
	if out2 != [2]byte{0x11, 0x22} {
		t.Fatalf("PutUint16 mismatch")
	}

	b4 := []byte{0x01, 0x02, 0x03, 0x04}
	u4 := o.Uint32(b4)
	var tmp4 [4]byte
	o.PutUint32(tmp4[:], u4)
	if tmp4 != [4]byte{0x01, 0x02, 0x03, 0x04} {
		t.Fatalf("PutUint32 roundtrip mismatch")
	}

	b8 := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80}
	u8 := o.Uint64(b8)
	var tmp8 [8]byte
	o.PutUint64(tmp8[:], u8)
	if tmp8 == [8]byte{} {
		t.Fatalf("PutUint64 should write bytes")
	}

	if o.name() != "TEST" {
		t.Fatalf("name mismatch")
	}
	if o.String() != "TEST" {
		t.Fatalf("String mismatch")
	}
}

func TestByteOrderNameAndErrorWrap(t *testing.T) {
	if got := byteOrderName(nil); got != "nil" {
		t.Fatalf("expect nil, got %s", got)
	}
	if got := byteOrderName(binary.BigEndian); got != "BigEndian" {
		t.Fatalf("expect BigEndian, got %s", got)
	}
	if got := byteOrderName(binary.LittleEndian); got != "LittleEndian" {
		t.Fatalf("expect LittleEndian, got %s", got)
	}
	if got := byteOrderName(OrderABCD); got == "" {
		t.Fatalf("expect non-empty name for custom order")
	}

	raw := []byte{0x01, 0x02}
	err := wrapError("decode", FormatSignedInt, binary.BigEndian, raw, fmt.Errorf("test"))
	var dfErr *Error
	if !errors.As(err, &dfErr) {
		t.Fatalf("expect wrapped Error type")
	}
	if dfErr.Inner == nil || dfErr.Op != "decode" || dfErr.Format != FormatSignedInt {
		t.Fatalf("wrapped fields mismatch")
	}
	if dfErr.Error() == "" {
		t.Fatalf("expect non-empty error string")
	}
	if unwrapped := errors.Unwrap(dfErr); unwrapped == nil {
		t.Fatalf("unwrap should return inner error")
	}
}

func TestParseIntegerAndUnsignedLengthErrors(t *testing.T) {
	if _, err := parseInteger([]byte{0x01, 0x02, 0x03}, binary.BigEndian, true); err == nil {
		t.Fatalf("expect error for invalid length signed int")
	}
	if _, err := parseUnsigned([]byte{0x01, 0x02, 0x03}, binary.BigEndian); err == nil {
		t.Fatalf("expect error for invalid length unsigned int")
	}
}

func TestSafeOpsOverflowAndNormal(t *testing.T) {
	if v, err := safeAdd(1, 2); err != nil || v != 3 {
		t.Fatalf("safeAdd normal failed")
	}
	if _, err := safeAdd(math.MaxInt64, 1); err == nil {
		t.Fatalf("expect overflow in safeAdd")
	}

	if v, err := safeSub(3, 1); err != nil || v != 2 {
		t.Fatalf("safeSub normal failed")
	}
	if _, err := safeSub(math.MinInt64, 1); err == nil {
		t.Fatalf("expect overflow in safeSub")
	}

	if v, err := safeMul(2, 3); err != nil || v != 6 {
		t.Fatalf("safeMul normal failed")
	}
	if _, err := safeMul(math.MaxInt64, 2); err == nil {
		t.Fatalf("expect overflow in safeMul")
	}

	if v, err := safeDiv(10, 2); err != nil || v != 5 {
		t.Fatalf("safeDiv normal failed")
	}
	if _, err := safeDiv(1, 0); err == nil {
		t.Fatalf("expect divide by zero in safeDiv")
	}
	if _, err := safeDiv(math.MinInt64, -1); err == nil {
		t.Fatalf("expect overflow in safeDiv")
	}

	if v, err := safeShl(1, 3); err != nil || v != 8 {
		t.Fatalf("safeShl normal failed")
	}
	if _, err := safeShl(1, 63); err == nil {
		t.Fatalf("expect shift overflow in safeShl")
	}
}

func BenchmarkFormatPoint_Int16(b *testing.B) {
	raw := []byte{0x12, 0x34}
	for i := 0; i < b.N; i++ {
		_, _ = FormatPoint(raw, binary.BigEndian, FormatSignedInt, "")
	}
}

func BenchmarkFormatPoint_Float32Expr(b *testing.B) {
	raw := []byte{0x00, 0x64}
	for i := 0; i < b.N; i++ {
		_, _ = FormatPoint(raw, binary.BigEndian, FormatExpr16, "v*2+1")
	}
}

func TestFormatPoint_ModbusPresets(t *testing.T) {
	type intCase struct {
		name  string
		order binary.ByteOrder
		ft    FormatType
		value int64
		bytes int
	}

	intCases := []intCase{
		{"Signed", binary.BigEndian, FormatSignedInt, -12345, 2},
		{"Unsigned", binary.BigEndian, FormatUnsignedInt, 54321, 2},
		{"LongABCD", OrderABCD, FormatSignedInt, 0x11223344, 4},
		{"LongCDAB", OrderCDAB, FormatSignedInt, 0x11223344, 4},
		{"LongBADC", OrderBADC, FormatSignedInt, 0x11223344, 4},
		{"LongDCBA", OrderDCBA, FormatSignedInt, 0x11223344, 4},
	}

	for _, c := range intCases {
		if c.bytes == 2 {
			raw := make([]byte, 2)
			c.order.PutUint16(raw, uint16(c.value))
			s, err := FormatPoint(raw, c.order, c.ft, "")
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", c.name, err)
			}
			v, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				t.Fatalf("%s: parse int error: %v", c.name, err)
			}
			if v != c.value {
				t.Fatalf("%s: expect %d, got %d", c.name, c.value, v)
			}
		} else {
			raw := make([]byte, 4)
			c.order.PutUint32(raw, uint32(c.value))
			s, err := FormatPoint(raw, c.order, c.ft, "")
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", c.name, err)
			}
			v, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				t.Fatalf("%s: parse int error: %v", c.name, err)
			}
			if v != c.value {
				t.Fatalf("%s: expect %d, got %d", c.name, c.value, v)
			}
		}
	}

	type floatCase struct {
		name  string
		order binary.ByteOrder
		ft    FormatType
		value float64
		bytes int
	}

	floatCases := []floatCase{
		{"FloatABCD", OrderABCD, FormatFloat32, 123.456, 4},
		{"FloatCDAB", OrderCDAB, FormatFloat32, 123.456, 4},
		{"FloatBADC", OrderBADC, FormatFloat32, 123.456, 4},
		{"FloatDCBA", OrderDCBA, FormatFloat32, 123.456, 4},
		{"DoubleABCDEFGH", OrderABCD, FormatFloat64, 12345678.5, 8},
		{"DoubleGHEFCDAB", OrderCDAB, FormatFloat64, 12345678.5, 8},
		{"DoubleBADCFEHG", OrderBADC, FormatFloat64, 12345678.5, 8},
		{"DoubleHGFEDCBA", OrderDCBA, FormatFloat64, 12345678.5, 8},
	}

	for _, c := range floatCases {
		if c.bytes == 4 {
			raw := make([]byte, 4)
			bits := math.Float32bits(float32(c.value))
			c.order.PutUint32(raw, bits)
			s, err := FormatPoint(raw, c.order, c.ft, "")
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", c.name, err)
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				t.Fatalf("%s: parse float error: %v", c.name, err)
			}
			if diff := math.Abs(c.value - v); diff > 1e-3 {
				t.Fatalf("%s: diff too large: %v", c.name, diff)
			}
		} else {
			raw := make([]byte, 8)
			bits := math.Float64bits(c.value)
			c.order.PutUint64(raw, bits)
			s, err := FormatPoint(raw, c.order, c.ft, "")
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", c.name, err)
			}
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				t.Fatalf("%s: parse float error: %v", c.name, err)
			}
			if diff := math.Abs(c.value - v); diff > 1e-6 {
				t.Fatalf("%s: diff too large: %v", c.name, diff)
			}
		}
	}

	raw := make([]byte, 2)
	binary.BigEndian.PutUint16(raw, 0x4167)
	if s, err := FormatPoint(raw, binary.BigEndian, FormatSignedInt, ""); err != nil || s == "" {
		t.Fatalf("Signed register sanity failed: %v, %s", err, s)
	}
	if s, err := FormatPoint(raw, binary.BigEndian, FormatUnsignedInt, ""); err != nil || s == "" {
		t.Fatalf("Unsigned register sanity failed: %v, %s", err, s)
	}
	if s, err := FormatPoint(raw, binary.BigEndian, FormatHexString, ""); err != nil || !strings.HasPrefix(s, "0x") {
		t.Fatalf("Hex register format failed: %v, %s", err, s)
	}
	if s, err := FormatPoint(raw, binary.BigEndian, FormatBinaryString, ""); err != nil || len(s) == 0 {
		t.Fatalf("Binary register format failed: %v, %s", err, s)
	}
}
