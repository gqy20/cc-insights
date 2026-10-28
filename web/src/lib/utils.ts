import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

// shadcn/ui 标准的 className 合并工具。
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Qoder 的 model 是内部别名，真实名称在 model_name 里；其他 provider 不下发该字段。
export function modelLabel(model: string, modelName?: string): string {
  return modelName || model
}

// 过滤候选里要同时给出真名和别名：后端 --model 匹配的是别名，只写真名就没法下钻。
export function modelOptionLabel(model: string, modelName?: string): string {
  return modelName && modelName !== model ? `${modelName} (${model})` : model
}
