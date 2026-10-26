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
export const preview = (path, mapping) => Go.Preview(path, plain(mapping))
export const loadMapping = (path) => Go.LoadMapping(path)
export const loadTarget = (path) => Go.LoadTarget(path)
export const saveMapping = (mapping, path, targetPath) => Go.SaveMapping(plain(mapping), path, targetPath)
export const convert = (mapping, input, output) => Go.Convert(plain(mapping), input, output)
export const platforms = () => Go.Platforms()
export const exportConverter = (mapping, os, arch, dir) => Go.Export(plain(mapping), os, arch, dir)
export const version = () => Go.Version()
export const recent = () => Go.Recent()
export const remember = (kind, path) => Go.Remember(kind, path).catch(() => {})
export const onFileDrop = (fn) => OnFileDrop((x, y, paths) => fn(paths), false)
