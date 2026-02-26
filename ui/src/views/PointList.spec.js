import { describe, it, expect } from 'vitest'
import baseTemplates from '@/utils/pointTemplates.json'
import {
  getWordOrderOptionsForBytes,
  filterParseTypesByBytes,
  reorderBytes,
  parseByType,
  applyFormula,
  wordOrderToBackend,
  registersToBytes
} from '@/utils/pointDecodeHelper'

describe('WordOrder and parse type helpers', () => {
  it('filters wordOrder by byte length', () => {
    let options = getWordOrderOptionsForBytes(1)
    expect(options.length).toBe(0)

    options = getWordOrderOptionsForBytes(2)
    const values2 = options.map(o => o.value)
    expect(values2).toEqual(['AB', 'BA'])

    options = getWordOrderOptionsForBytes(4)
    const values4 = options.map(o => o.value)
    expect(values4).toContain('ABCD')
    expect(values4).toContain('DCBA')
  })

  it('filters parse types by byte length', () => {
    let options = filterParseTypesByBytes(1)
    expect(options.find(o => o.value === 'UINT8')).toBeTruthy()
    expect(options.find(o => o.value === 'UINT16')).toBeFalsy()

    options = filterParseTypesByBytes(2)
    expect(options.find(o => o.value === 'UINT16')).toBeTruthy()
    expect(options.find(o => o.value === 'UINT32')).toBeFalsy()
  })

  it('reorders bytes according to wordOrder', () => {
    const src = [0x01, 0x02, 0x03, 0x04]
    const ba = reorderBytes(src, 2, 'BA')
    expect(ba).toEqual([0x02, 0x01])
    const dcba = reorderBytes([...src, 0x05, 0x06, 0x07, 0x08], 4, 'DCBA')
    expect(dcba.slice(0, 4)).toEqual([0x07, 0x08, 0x05, 0x06])
  })

  it('parses bytes by type and applies formula', () => {
    const bytes = [0x00, 0x64]
    const raw = parseByType(bytes, 'UINT16')
    expect(raw).toBe(100)
    const engineered = applyFormula(raw, 'v * 0.1', 1, 0)
    expect(engineered).toBe(10)
  })

  it('maps wordOrder to backend format', () => {
    expect(wordOrderToBackend('AB')).toBe('ABCD')
    expect(wordOrderToBackend('BA')).toBe('DCBA')
    expect(wordOrderToBackend('ABCD')).toBe('ABCD')
  })

  it('merges registers into big-endian bytes', () => {
    const regs = [0x1122, 0x3344]
    const bytes = registersToBytes(regs)
    expect(bytes).toEqual([0x11, 0x22, 0x33, 0x44])
  })

  it('handles invalid register values and masks to 16 bits', () => {
    const regs = [65537, 'abc', 0xFFEE]
    const bytes = registersToBytes(regs)
    expect(bytes).toEqual([0x00, 0x01, 0xFF, 0xEE])
  })

  it('loads BACnet and OPC UA templates', () => {
    const protocols = new Set(baseTemplates.map(t => t.protocol))
    expect(protocols.has('bacnet-ip')).toBe(true)
    expect(protocols.has('opc-ua')).toBe(true)
    expect(baseTemplates.length).toBeGreaterThanOrEqual(5)
  })
})
