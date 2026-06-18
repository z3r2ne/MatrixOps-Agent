# 仿真视图 2.5D 图片资源说明

本文档用于指导 AI 图片生成工具批量产出「办公仿真视图」所需的等距（Isometric）2.5D 精灵图。

生成完成后，将所有 PNG 文件保存到本目录：

```
frontend/public/assets/simulation/
```

---

## 一、通用技术规格


| 项目  | 要求                          |
| --- | --------------------------- |
| 格式  | PNG，透明背景                    |
| 视角  | 等距 2.5D，相机约 30° 俯视，2:1 等距比例 |
| 光照  | 统一从左上打光，右下阴影                |
| 风格  | 现代科技公司办公室，轻量游戏/UI 资产风       |
| 色调  | 浅灰蓝地板 + 白/浅木色家具 + 少量绿植点缀    |
| 建议  | 先生成 2x 分辨率，确认风格后再批量生成       |


### 尺寸与比例规范

所有资源以 **1x 基准像素** 为准；若生成 2x，则各边等比放大 2 倍，**比例不变**。

#### 画布宽高比（宽 : 高）


| 类型     | 宽高比                    | 说明                 |
| ------ | ---------------------- | ------------------ |
| 等距地板砖  | **2 : 1**              | 经典等距菱形地砖比例         |
| 标准工位   | **8 : 7**（≈1.14 : 1）   | 略宽于高的精灵图           |
| 主工位    | **20 : 17**（≈1.18 : 1） | 比标准工位宽约 25%、高约 21% |
| 机器人    | **3 : 4**              | 竖向半身角色             |
| 办公室背景  | **16 : 9**             | 宽屏背景               |
| 隔断墙    | **5 : 4**              | 横向模块               |
| 状态光晕   | **4 : 3**              | 与显示器屏幕形状匹配         |
| 完成标记   | **1 : 1**              | 正方形图标              |
| 放大显示器框 | **3 : 2**              | 横向大显示器             |


#### 资产相对缩放（以标准工位为 1.0）


| 资产                         | 相对宽度     | 相对高度     | 备注                |
| -------------------------- | -------- | -------- | ----------------- |
| `floor-tile.png`           | 0.80     | 0.46     | 单块地砖宽度 ≈ 标准工位 80% |
| `workstation-standard.png` | **1.00** | **1.00** | 基准                |
| `workstation-lead.png`     | **1.25** | **1.21** | 主任务工位，视觉更突出       |
| `robot-*-idle.png`         | 0.30     | 0.46     | 叠在工位椅子上，高约工位 46%  |
| `partition-wall.png`       | 0.63     | 0.57     | 约为工位宽 63%         |
| `decor-plant.png`          | 0.25     | 0.36     | 小装饰               |
| `decor-coffee.png`         | 0.38     | 0.50     | 中等装饰              |
| `decor-server-rack.png`    | 0.31     | 0.64     | 竖向道具              |
| `decor-whiteboard.png`     | 0.50     | 0.43     | 墙面装饰              |
| `status-running-glow.png`  | 0.50     | 0.43     | 与屏幕区域同尺寸叠放        |
| `status-done-check.png`    | 0.15     | 0.17     | 角标气泡              |
| `monitor-focus-frame.png`  | 2.81     | 2.14     | 全屏放大弹层用           |


#### 场景布局间距（前端摆放参考）


| 参数      | 比例 / 数值              | 说明                |
| ------- | -------------------- | ----------------- |
| 工位水平间距  | 标准工位宽 × **1.15**     | 相邻工位中心点水平距离       |
| 工位垂直间距  | 标准工位高 × **1.20**     | 相邻行工位中心点垂直距离      |
| 机器人叠放锚点 | 工位底边中点向上 **18%** 工位高 | 机器人脚底对齐椅子区域       |
| 屏幕文字叠放区 | 见各工位「屏幕区域比例」         | HTML 内容覆盖在深色屏幕矩形上 |


#### 可交互区域 — 屏幕叠放比例（占整张图）

生成时尽量让深色屏幕区域落在以下比例范围内，前端将按此叠 Agent 输出：

`**workstation-standard.png`（单屏）**


| 区域     | 左偏移     | 上偏移     | 宽度      | 高度      |
| ------ | ------- | ------- | ------- | ------- |
| 主显示器屏幕 | **28%** | **10%** | **38%** | **22%** |


`**workstation-lead.png`（双屏）**


| 区域  | 左偏移     | 上偏移    | 宽度      | 高度      |
| --- | ------- | ------ | ------- | ------- |
| 左屏  | **18%** | **8%** | **28%** | **20%** |
| 右屏  | **50%** | **8%** | **28%** | **20%** |


`**monitor-focus-frame.png`（放大弹层）**


| 区域   | 左偏移     | 上偏移     | 宽度      | 高度      |
| ---- | ------- | ------- | ------- | ------- |
| 中央屏幕 | **10%** | **12%** | **80%** | **72%** |


`**office-backdrop.png`（背景留空区）**


| 区域        | 左偏移     | 上偏移     | 宽度      | 高度      |
| --------- | ------- | ------- | ------- | ------- |
| 可摆放工位的地板区 | **10%** | **35%** | **80%** | **55%** |


> 比例以图片左上角为 (0%, 0%)，向右为宽、向下为高。若 AI 生成结果有偏差，交付时请注明实际测量值。

### 全局风格关键词（每条提示词末尾追加）

```
isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, transparent background, no text, no watermark, high detail edges, UI game sprite
```

### 全局负面提示词

```
realistic photo, 3D render, perspective distortion, text, logo, watermark, blurry, cropped, busy background, people faces, anime, pixel art, dark horror, cluttered
```

### 风格参考说明（可选附给生成工具）

> 参考轻量等距模拟经营游戏（如 SimCity BuildIt、Townscaper 办公场景、Habbo Hotel 等距房间），但不要像素风，要矢量感 + 轻 3D 阴影。

---

## 二、资源清单

### P0 — 必须优先生成

---

#### 1. 地板地砖（可平铺）


| 项目         | 内容                                               |
| ---------- | ------------------------------------------------ |
| **保存文件名**  | `floor-tile.png`                                 |
| **建议尺寸**   | 256 × 128 px（1x）/ 512 × 256 px（2x）               |
| **宽高比**    | **2 : 1**                                        |
| **画布内容占比** | 菱形地砖占画布面积约 **85%**，四角透明留边约 7.5%                  |
| **相对缩放**   | 宽度 = 标准工位的 **80%**，高度 = 标准工位的 **46%**            |
| **平铺间距**   | 水平步进 = 砖宽 × **100%**，垂直步进 = 砖高 × **50%**（等距错位铺砖） |
| **用途**     | 场景底层平铺地板                                         |


**完整提示词：**

```
A single isometric floor tile for a modern open-plan tech office, light warm-gray carpet texture with subtle grid lines, diamond-shaped isometric tile, seamless tileable edges on all sides, soft ambient occlusion at tile seams, minimal wear, clean corporate style. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, transparent background outside the diamond tile shape, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 2. 标准工位（桌子 + 显示器，屏幕留空）


| 项目         | 内容                                                      |
| ---------- | ------------------------------------------------------- |
| **保存文件名**  | `workstation-standard.png`                              |
| **建议尺寸**   | 320 × 280 px（1x）/ 640 × 560 px（2x）                      |
| **宽高比**    | **8 : 7**（≈1.14 : 1）                                    |
| **相对缩放**   | 宽度 **1.00**、高度 **1.00**（全场景基准）                          |
| **屏幕区域比例** | 左 **28%**、上 **10%**、宽 **38%**、高 **22%**（深色空白矩形 #1a1f2e） |
| **内容留白**   | 精灵主体居中，左右透明边各约 **5%**，底部阴影不超出画布                         |
| **用途**     | 子任务 Agent 工位；显示器屏幕区域需为纯色深灰，供前端叠加文字                      |
| **关键要求**   | 屏幕必须是空白深色矩形（#1a1f2e），不能有任何文字或图案                         |


**完整提示词：**

```
Isometric 2.5D office workstation sprite: compact desk with keyboard, mouse, office chair behind desk, and a computer monitor facing the viewer. The monitor screen must be a flat empty dark rectangle (#1a1f2e) with no content, clearly defined bezel, ready for UI text overlay. Light oak or white desk, slim monitor, modern ergonomic chair. Viewed from classic isometric angle (30 degree top-down). Single object centered, transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 3. 主工位（根任务 / 组长位）


| 项目             | 内容                                     |
| -------------- | -------------------------------------- |
| **保存文件名**      | `workstation-lead.png`                 |
| **建议尺寸**       | 400 × 340 px（1x）/ 800 × 680 px（2x）     |
| **宽高比**        | **20 : 17**（≈1.18 : 1）                 |
| **相对缩放**       | 相对标准工位：宽 **125%**、高 **121%**           |
| **屏幕区域比例（左屏）** | 左 **18%**、上 **8%**、宽 **28%**、高 **20%** |
| **屏幕区域比例（右屏）** | 左 **50%**、上 **8%**、宽 **28%**、高 **20%** |
| **用途**         | 当前主任务（根任务）工位，比普通工位更大、更醒目               |
| **关键要求**       | 双屏或宽桌；两块屏幕均为空白深色矩形（#1a1f2e）            |


**完整提示词：**

```
Isometric 2.5D lead developer workstation sprite, larger than standard desk: L-shaped or wide desk with dual monitors side by side, both screens are empty flat dark rectangles (#1a1f2e) with no content, premium office chair, small desk plant, notebook. Slightly more prominent and authoritative than a regular cubicle. Classic isometric game asset angle, transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 4. 机器人角色 — 蓝色（坐姿）


| 项目        | 内容                                   |
| --------- | ------------------------------------ |
| **保存文件名** | `robot-blue-idle.png`                |
| **建议尺寸**  | 96 × 128 px（1x）/ 192 × 256 px（2x）    |
| **宽高比**   | **3 : 4**                            |
| **相对缩放**  | 宽 = 标准工位 **30%**，高 = 标准工位 **46%**    |
| **叠放锚点**  | 底部中心对齐工位椅子区域，脚底距工位底边向上约 **18%** 工位高  |
| **角色占比**  | 机器人本体占画布高约 **75%**，头顶向上留透明边约 **12%** |
| **用途**    | 蓝色主题 Agent 机器人，坐在工位椅子上               |


**完整提示词：**

```
Cute isometric 2.5D robot office worker character, sitting on office chair facing computer, small rounded robot body with antenna, glowing blue accent lights, friendly minimal face (simple LED eyes, no human face), compact proportions suitable for game sprite overlay on desk scene. Idle pose, arms near keyboard. Transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 5. 机器人角色 — 紫色（坐姿）


| 项目        | 内容                                          |
| --------- | ------------------------------------------- |
| **保存文件名** | `robot-purple-idle.png`                     |
| **建议尺寸**  | 96 × 128 px（1x）/ 192 × 256 px（2x）           |
| **宽高比**   | **3 : 4**（与 `robot-blue-idle.png` 完全一致，仅换色） |
| **相对缩放**  | 同蓝色机器人                                      |
| **用途**    | 紫色主题 Agent 机器人                              |


**完整提示词：**

```
Cute isometric 2.5D robot office worker character, sitting on office chair facing computer, small rounded robot body with antenna, glowing purple accent lights, friendly minimal face (simple LED eyes, no human face), compact proportions suitable for game sprite overlay on desk scene. Idle pose, arms near keyboard. Transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 6. 机器人角色 — 绿色（坐姿）


| 项目        | 内容                                          |
| --------- | ------------------------------------------- |
| **保存文件名** | `robot-green-idle.png`                      |
| **建议尺寸**  | 96 × 128 px（1x）/ 192 × 256 px（2x）           |
| **宽高比**   | **3 : 4**（与 `robot-blue-idle.png` 完全一致，仅换色） |
| **相对缩放**  | 同蓝色机器人                                      |
| **用途**    | 绿色主题 Agent 机器人                              |


**完整提示词：**

```
Cute isometric 2.5D robot office worker character, sitting on office chair facing computer, small rounded robot body with antenna, glowing green accent lights, friendly minimal face (simple LED eyes, no human face), compact proportions suitable for game sprite overlay on desk scene. Idle pose, arms near keyboard. Transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 7. 机器人角色 — 橙色（坐姿）


| 项目        | 内容                                          |
| --------- | ------------------------------------------- |
| **保存文件名** | `robot-orange-idle.png`                     |
| **建议尺寸**  | 96 × 128 px（1x）/ 192 × 256 px（2x）           |
| **宽高比**   | **3 : 4**（与 `robot-blue-idle.png` 完全一致，仅换色） |
| **相对缩放**  | 同蓝色机器人                                      |
| **用途**    | 橙色主题 Agent 机器人                              |


**完整提示词：**

```
Cute isometric 2.5D robot office worker character, sitting on office chair facing computer, small rounded robot body with antenna, glowing orange accent lights, friendly minimal face (simple LED eyes, no human face), compact proportions suitable for game sprite overlay on desk scene. Idle pose, arms near keyboard. Transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 8. 机器人角色 — 粉色（坐姿）


| 项目        | 内容                                          |
| --------- | ------------------------------------------- |
| **保存文件名** | `robot-pink-idle.png`                       |
| **建议尺寸**  | 96 × 128 px（1x）/ 192 × 256 px（2x）           |
| **宽高比**   | **3 : 4**（与 `robot-blue-idle.png` 完全一致，仅换色） |
| **相对缩放**  | 同蓝色机器人                                      |
| **用途**    | 粉色主题 Agent 机器人                              |


**完整提示词：**

```
Cute isometric 2.5D robot office worker character, sitting on office chair facing computer, small rounded robot body with antenna, glowing pink accent lights, friendly minimal face (simple LED eyes, no human face), compact proportions suitable for game sprite overlay on desk scene. Idle pose, arms near keyboard. Transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 9. 机器人角色 — 青色（坐姿）


| 项目        | 内容                                          |
| --------- | ------------------------------------------- |
| **保存文件名** | `robot-cyan-idle.png`                       |
| **建议尺寸**  | 96 × 128 px（1x）/ 192 × 256 px（2x）           |
| **宽高比**   | **3 : 4**（与 `robot-blue-idle.png` 完全一致，仅换色） |
| **相对缩放**  | 同蓝色机器人                                      |
| **用途**    | 青色主题 Agent 机器人                              |


**完整提示词：**

```
Cute isometric 2.5D robot office worker character, sitting on office chair facing computer, small rounded robot body with antenna, glowing cyan accent lights, friendly minimal face (simple LED eyes, no human face), compact proportions suitable for game sprite overlay on desk scene. Idle pose, arms near keyboard. Transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

### P1 — 强烈建议（增强场景感）

---

#### 10. 办公室背景墙


| 项目           | 内容                                                       |
| ------------ | -------------------------------------------------------- |
| **保存文件名**    | `office-backdrop.png`                                    |
| **建议尺寸**     | 1920 × 1080 px（1x）/ 3840 × 2160 px（2x）                   |
| **宽高比**      | **16 : 9**                                               |
| **可摆放区域**    | 左 **10%**、上 **35%**、宽 **80%**、高 **55%**（此区域内不绘制家具，供叠加工位） |
| **远景 / 近景比** | 窗户与墙面约占画面上方 **35%**，地板留空区约占下方 **55%**                    |
| **用途**       | 场景最底层背景；中央留空以便叠加工位精灵                                     |


**完整提示词：**

```
Wide isometric 2.5D modern tech office interior backdrop, open floor plan with glass walls in background, soft daylight from large windows, distant city skyline blur, light blue-gray walls, ceiling with recessed lights, empty floor area in center foreground (no furniture) so game sprites can be placed on top. Panoramic composition, atmospheric depth, no characters, no desks. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 11. 办公隔断墙


| 项目        | 内容                                 |
| --------- | ---------------------------------- |
| **保存文件名** | `partition-wall.png`               |
| **建议尺寸**  | 200 × 160 px（1x）/ 400 × 320 px（2x） |
| **宽高比**   | **5 : 4**                          |
| **相对缩放**  | 宽 = 标准工位 **63%**，高 = 标准工位 **57%**  |
| **用途**    | 工位区域分隔装饰                           |


**完整提示词：**

```
Isometric 2.5D low office partition wall module, frosted glass top half and white frame, modular cubicle divider, single segment, transparent background, game tile asset. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 12. 装饰 — 绿植


| 项目        | 内容                                |
| --------- | --------------------------------- |
| **保存文件名** | `decor-plant.png`                 |
| **建议尺寸**  | 80 × 100 px（1x）/ 160 × 200 px（2x） |
| **宽高比**   | **4 : 5**                         |
| **相对缩放**  | 宽 = 标准工位 **25%**，高 = 标准工位 **36%** |
| **用途**    | 角落绿植装饰                            |


**完整提示词：**

```
Small isometric 2.5D office potted plant decoration sprite, monstera in white ceramic pot, cute scale, transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 13. 装饰 — 咖啡机


| 项目        | 内容                                 |
| --------- | ---------------------------------- |
| **保存文件名** | `decor-coffee.png`                 |
| **建议尺寸**  | 120 × 140 px（1x）/ 240 × 280 px（2x） |
| **宽高比**   | **6 : 7**（≈0.86 : 1）               |
| **相对缩放**  | 宽 = 标准工位 **38%**，高 = 标准工位 **50%**  |
| **用途**    | 茶水间咖啡机装饰                           |


**完整提示词：**

```
Isometric 2.5D office coffee machine decoration sprite, modern espresso machine on small counter table, compact size, transparent background, game environment prop. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 14. 装饰 — 服务器机柜


| 项目        | 内容                                 |
| --------- | ---------------------------------- |
| **保存文件名** | `decor-server-rack.png`            |
| **建议尺寸**  | 100 × 180 px（1x）/ 200 × 360 px（2x） |
| **宽高比**   | **5 : 9**（竖向）                      |
| **相对缩放**  | 宽 = 标准工位 **31%**，高 = 标准工位 **64%**  |
| **用途**    | 服务器机柜装饰，暗示 AI 基础设施                 |


**完整提示词：**

```
Isometric 2.5D server rack decoration sprite for tech office, black rack with blinking blue LED lights, glass door, compact game prop scale, transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 15. 装饰 — 白板


| 项目         | 内容                                                 |
| ---------- | -------------------------------------------------- |
| **保存文件名**  | `decor-whiteboard.png`                             |
| **建议尺寸**   | 160 × 120 px（1x）/ 320 × 240 px（2x）                 |
| **宽高比**    | **4 : 3**                                          |
| **相对缩放**   | 宽 = 标准工位 **50%**，高 = 标准工位 **43%**                  |
| **白板区域比例** | 左 **12%**、上 **15%**、宽 **76%**、高 **58%**（空白浅色，可叠文字） |
| **关键要求**   | 白板区域保持空白浅色，便于后期叠加任务名称                              |


**完整提示词：**

```
Isometric 2.5D office whiteboard decoration sprite on stand, empty clean white board surface with no writing, silver frame, marker tray, transparent background, game prop. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

### P2 — 交互与状态（第二阶段）

---

#### 16. 状态光晕 — 执行中


| 项目        | 内容                                                            |
| --------- | ------------------------------------------------------------- |
| **保存文件名** | `status-running-glow.png`                                     |
| **建议尺寸**  | 160 × 120 px（1x）/ 320 × 240 px（2x）                            |
| **宽高比**   | **4 : 3**                                                     |
| **相对缩放**  | 与标准工位单屏区域同尺寸（宽 **50%** 工位、高 **43%** 工位）                       |
| **叠放对齐**  | 中心对齐 `workstation-standard` 屏幕区域，中空部分宽 **60%**、高 **50%** 保持透明 |
| **用途**    | 任务执行中时叠在显示器上方                                                 |


**完整提示词：**

```
Isometric 2.5D subtle blue glow aura ring sprite for highlighting active computer monitor, soft neon cyan bloom, hollow center (transparent inside), elliptical shape matching monitor screen from isometric angle, transparent background, UI VFX sprite. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 17. 状态光晕 — 失败


| 项目        | 内容                                                |
| --------- | ------------------------------------------------- |
| **保存文件名** | `status-error-glow.png`                           |
| **建议尺寸**  | 160 × 120 px（1x）/ 320 × 240 px（2x）                |
| **宽高比**   | **4 : 3**（与 `status-running-glow.png` 完全一致，仅换色为红） |
| **叠放对齐**  | 同执行中光晕                                            |
| **用途**    | 任务失败时叠在显示器上方                                      |


**完整提示词：**

```
Isometric 2.5D subtle red glow aura ring sprite for highlighting failed computer monitor, soft neon red bloom, hollow center (transparent inside), elliptical shape matching monitor screen from isometric angle, transparent background, UI VFX sprite. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 18. 状态标记 — 完成


| 项目        | 内容                                 |
| --------- | ---------------------------------- |
| **保存文件名** | `status-done-check.png`            |
| **建议尺寸**  | 48 × 48 px（1x）/ 96 × 96 px（2x）     |
| **宽高比**   | **1 : 1**                          |
| **相对缩放**  | 宽 = 标准工位 **15%**                   |
| **叠放位置**  | 对齐工位右上角，距右 **5%**、距上 **8%**（相对工位图） |
| **用途**    | 任务完成角标                             |


**完整提示词：**

```
Small isometric 2.5D green checkmark status badge sprite, rounded bubble with white check icon, soft green glow, game UI notification icon, transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 19. 放大显示器边框


| 项目         | 内容                                               |
| ---------- | ------------------------------------------------ |
| **保存文件名**  | `monitor-focus-frame.png`                        |
| **建议尺寸**   | 900 × 600 px（1x）/ 1800 × 1200 px（2x）             |
| **宽高比**    | **3 : 2**                                        |
| **相对缩放**   | 宽 = 标准工位 **2.81** 倍，高 = 标准工位 **2.14** 倍          |
| **屏幕区域比例** | 左 **10%**、上 **12%**、宽 **80%**、高 **72%**（中央深色空白区） |
| **边框占比**   | 显示器边框 + 底座约占画布外围 **10%～15%**                     |
| **用途**     | 点击工位放大时使用的显示器边框；中央大面积留空供聊天内容展示                   |
| **关键要求**   | 屏幕中央约 80% 宽 × 72% 高为深色空白，四周为显示器边框                |


**完整提示词：**

```
Large isometric-angled computer monitor close-up frame for UI overlay, thick bezel, empty dark screen area in center (80% of image) for chat content, subtle desk reflection below, cinematic focus feel, transparent outside monitor shape. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, no text on screen, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 20. 机器人工作动画 — 蓝色帧 1


| 项目        | 内容                                        |
| --------- | ----------------------------------------- |
| **保存文件名** | `robot-blue-working-1.png`                |
| **建议尺寸**  | 96 × 128 px（1x）/ 192 × 256 px（2x）         |
| **宽高比**   | **3 : 4**（与 `robot-blue-idle.png` 画布完全一致） |
| **相对缩放**  | 同 idle 姿态，仅手臂动作不同                         |
| **用途**    | 打字动画第 1 帧（可选，后续实现）                        |


**完整提示词：**

```
Cute isometric 2.5D robot office worker character, sitting on office chair facing computer, small rounded robot body with antenna, glowing blue accent lights, friendly minimal face (simple LED eyes, no human face). Working pose frame 1: right arm raised slightly above keyboard as if typing. Transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

#### 21. 机器人工作动画 — 蓝色帧 2


| 项目        | 内容                                        |
| --------- | ----------------------------------------- |
| **保存文件名** | `robot-blue-working-2.png`                |
| **建议尺寸**  | 96 × 128 px（1x）/ 192 × 256 px（2x）         |
| **宽高比**   | **3 : 4**（与 `robot-blue-idle.png` 画布完全一致） |
| **相对缩放**  | 同 idle 姿态，仅手臂动作不同                         |
| **用途**    | 打字动画第 2 帧（可选，后续实现）                        |


**完整提示词：**

```
Cute isometric 2.5D robot office worker character, sitting on office chair facing computer, small rounded robot body with antenna, glowing blue accent lights, friendly minimal face (simple LED eyes, no human face). Working pose frame 2: both arms lowered on keyboard actively typing. Transparent background. isometric 2.5D game asset, modern tech office, clean flat illustration, soft pastel colors, top-left lighting, subtle drop shadow, no text, no watermark, high detail edges, UI game sprite
```

**负面提示词：** 使用本文档「全局负面提示词」

---

## 三、推荐生成顺序

### 第 1 批（验证风格，最小可用）

1. `floor-tile.png`
2. `workstation-standard.png`
3. `workstation-lead.png`
4. `robot-blue-idle.png`

确认视角、光照、风格一致后，再批量生成其余资源。

### 第 2 批（补全角色与背景）

1. `robot-purple-idle.png`
2. `robot-green-idle.png`
3. `robot-orange-idle.png`
4. `robot-pink-idle.png`
5. `robot-cyan-idle.png`
6. `office-backdrop.png`

### 第 3 批（场景装饰与交互）

1. `partition-wall.png`
2. `decor-plant.png`
3. `decor-coffee.png`
4. `decor-server-rack.png`
5. `decor-whiteboard.png`
6. `status-running-glow.png`
7. `status-error-glow.png`
8. `status-done-check.png`
9. `monitor-focus-frame.png`
10. `robot-blue-working-1.png`（可选）
11. `robot-blue-working-2.png`（可选）

---

## 四、生成完成后的交付信息

图片放入本目录后，请同步提供以下信息，便于前端精确对齐屏幕区域：

1. **每张图的实际像素尺寸**（宽 × 高）
2. **工位图中显示器屏幕区域**的大致位置（可用比例描述，例如：屏幕左上角约在图片 32% 宽、18% 高处，占宽 36%、高 22%）
3. **所有工位与机器人是否视角一致**（若不一致需逐张标注）

---

## 五、文件名速查表


| 优先级 | 文件名                        | 1x 尺寸（宽×高）  | 宽高比   | 说明          |
| --- | -------------------------- | ----------- | ----- | ----------- |
| P0  | `floor-tile.png`           | 256 × 128   | 2:1   | 地板地砖        |
| P0  | `workstation-standard.png` | 320 × 280   | 8:7   | 标准工位（基准）    |
| P0  | `workstation-lead.png`     | 400 × 340   | 20:17 | 主工位（125% 宽） |
| P0  | `robot-blue-idle.png`      | 96 × 128    | 3:4   | 机器人（蓝）      |
| P0  | `robot-purple-idle.png`    | 96 × 128    | 3:4   | 机器人（紫）      |
| P0  | `robot-green-idle.png`     | 96 × 128    | 3:4   | 机器人（绿）      |
| P0  | `robot-orange-idle.png`    | 96 × 128    | 3:4   | 机器人（橙）      |
| P0  | `robot-pink-idle.png`      | 96 × 128    | 3:4   | 机器人（粉）      |
| P0  | `robot-cyan-idle.png`      | 96 × 128    | 3:4   | 机器人（青）      |
| P1  | `office-backdrop.png`      | 1920 × 1080 | 16:9  | 办公室背景       |
| P1  | `partition-wall.png`       | 200 × 160   | 5:4   | 隔断墙         |
| P1  | `decor-plant.png`          | 80 × 100    | 4:5   | 绿植          |
| P1  | `decor-coffee.png`         | 120 × 140   | 6:7   | 咖啡机         |
| P1  | `decor-server-rack.png`    | 100 × 180   | 5:9   | 服务器机柜       |
| P1  | `decor-whiteboard.png`     | 160 × 120   | 4:3   | 白板          |
| P2  | `status-running-glow.png`  | 160 × 120   | 4:3   | 执行中光晕       |
| P2  | `status-error-glow.png`    | 160 × 120   | 4:3   | 失败光晕        |
| P2  | `status-done-check.png`    | 48 × 48     | 1:1   | 完成标记        |
| P2  | `monitor-focus-frame.png`  | 900 × 600   | 3:2   | 放大显示器边框     |
| P2  | `robot-blue-working-1.png` | 96 × 128    | 3:4   | 工作动画帧 1     |
| P2  | `robot-blue-working-2.png` | 96 × 128    | 3:4   | 工作动画帧 2     |


