import { expect, test } from '@playwright/test'

// 只读冒烟：只断言页面标题、表头与静态文案，不依赖业务数据。
test.beforeEach(async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('menuitem', { name: '运营概览' })).toBeVisible()
})

test('运营概览展示关键指标', async ({ page }) => {
  await page.goto('/dashboard')
  const content = page.locator('.ant-layout-content')
  await expect(content.getByText('运营概览', { exact: true })).toBeVisible()
  await expect(content.getByText('题目总量', { exact: true })).toBeVisible()
  await expect(content.getByText('活跃会员', { exact: true })).toBeVisible()
  await expect(content.getByText('最近订单', { exact: true })).toBeVisible()
  await expect(content.getByText('7 天内到期会员', { exact: true })).toBeVisible()
})

test('题目管理展示列表与筛选', async ({ page }) => {
  await page.goto('/questions')
  const content = page.locator('.ant-layout-content')
  await expect(content.getByRole('heading', { name: '题目管理' })).toBeVisible()
  await expect(content.getByRole('button', { name: '新建题目' })).toBeVisible()
  await expect(content.getByPlaceholder('关键词')).toBeVisible()
  await expect(content.getByRole('columnheader', { name: '题干' })).toBeVisible()
  await expect(content.getByRole('columnheader', { name: '状态' })).toBeVisible()
})

test('专题题包展示入口与列表', async ({ page }) => {
  await page.goto('/topics')
  const content = page.locator('.ant-layout-content')
  await expect(content.getByRole('heading', { name: '专题题包' })).toBeVisible()
  await expect(content.getByRole('button', { name: '新建专题' })).toBeVisible()
  await expect(content.getByRole('columnheader', { name: '名称' })).toBeVisible()
})

test('会员套餐展示入口与列表', async ({ page }) => {
  await page.goto('/membership')
  const content = page.locator('.ant-layout-content')
  await expect(content.getByRole('heading', { name: '会员套餐' })).toBeVisible()
  await expect(content.getByRole('button', { name: '新建套餐' })).toBeVisible()
  await expect(content.getByText('价格按元展示，服务端以分存储')).toBeVisible()
})

test('家庭设备展示搜索与列表', async ({ page }) => {
  await page.goto('/families')
  const content = page.locator('.ant-layout-content')
  await expect(content.getByRole('heading', { name: '家庭与设备' })).toBeVisible()
  await expect(
    content.getByPlaceholder('家庭 ID / 孩子昵称 / 设备 ID'),
  ).toBeVisible()
  await expect(content.getByText('不展示手机号与设备令牌')).toBeVisible()
})

test('审计日志展示筛选与列表', async ({ page }) => {
  await page.goto('/audit')
  const content = page.locator('.ant-layout-content')
  await expect(content.getByRole('heading', { name: '审计日志' })).toBeVisible()
  await expect(content.getByPlaceholder('对象 ID')).toBeVisible()
  await expect(content.locator('.ant-select')).toHaveCount(2)
  await expect(content.getByRole('columnheader', { name: '时间' })).toBeVisible()
})
