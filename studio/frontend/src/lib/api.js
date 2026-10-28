// Thin wrapper over the Go methods bound by Wails (see studio/app.go).
// Every call takes plain data: Svelte state is snapshotted first, since the
// bridge serializes arguments as JSON.
import * as Go from '../../wailsjs/go/main/App'
import { OnFileDrop } from '../../wailsjs/runtime/runtime'

const plain = (v) => (v === undefined ? v : JSON.parse(JSON.stringify(v)))

export const chooseFile = (title, kind) => Go.ChooseFile(title, kind)
export const chooseSaveFile = (title, path, kind) => Go.ChooseSaveFile(title, path, kind)
export const chooseDirectory = (title) => Go.ChooseDirectory(title)
export const open = (path) => Go.Open(path)
export const readRaw = (path, input) => Go.ReadRaw(path, plain(input))
export const check = (mapping) => Go.Check(plain(mapping))
export const preview = (path, mapping, opts = {}) => Go.Preview(path, plain(mapping), opts)
export const batches = (path, mapping, by) => Go.Batches(path, plain(mapping), by)
export const checkFormula = (mapping, path, src) => Go.CheckFormula(plain(mapping), path, src)
let docs = null
export const functions = () => (docs ??= Go.Functions().catch(() => []))
export const loadMapping = (path) => Go.LoadMapping(path)
export const loadTarget = (path) => Go.LoadTarget(path)
export const draftTarget = (path) => Go.DraftTarget(path)
export const checkTarget = (target) => Go.CheckTarget(plain(target))
export const saveTarget = (target, path) => Go.SaveTarget(plain(target), path)
export const saveMapping = (mapping, path, targetPath, writeTarget) => Go.SaveMapping(plain(mapping), path, targetPath, writeTarget)
export const exists = (path) => Go.Exists(path)
export const convert = (mapping, input, output) => Go.Convert(plain(mapping), input, output)
export const platforms = () => Go.Platforms()
export const exportConverter = (mapping, os, arch, dir) => Go.Export(plain(mapping), os, arch, dir)
export const version = () => Go.Version()
export const recent = () => Go.Recent()
export const setDirty = (dirty) => Go.SetDirty(dirty).catch(() => {})
export const remember = (kind, path) => Go.Remember(kind, path).catch(() => {})
export const onFileDrop = (fn) => OnFileDrop((x, y, paths) => fn(paths), false)
