import React, { createContext, useContext, type Dispatch, type ReactNode, type SetStateAction } from "react"

/** Electron 工作台顶栏：设置按钮左侧的聊天工具区（由 ChatInterfaceV2 注册） */
export const WorkbenchChatTitleToolbarSetterContext = createContext<Dispatch<
  SetStateAction<ReactNode | null>
> | null>(null)

export function useWorkbenchChatTitleToolbarSetter(): Dispatch<SetStateAction<ReactNode | null>> | null {
  return useContext(WorkbenchChatTitleToolbarSetterContext)
}
