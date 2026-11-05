import { expect, test as setup } from '@playwright/test'
import { authFile } from '../playwright.config'

const username = process.env.ADMIN_USER || 'admin'
const password = process.env.ADMIN_PASSWORD || 'admin12345'

setup('管理员登录并保存会话', async ({ page }) => {
  await page.goto('/')

  // antd 会在两个汉字的按钮中插入空格（“登 录”），这里用正则匹配。
  const loginButton = page.getByRole('button', { name: /登\s*录/ })
  await expect(loginButton).toBeVisible()
  await page.getByPlaceholder('admin').fill(username)
  await page.getByPlaceholder('密码').fill(password)
  await loginButton.click()

  // 登录成功后进入题目管理，左侧菜单出现运营概览。
  await expect(page.getByRole('heading', { name: '题目管理' })).toBeVisible()
  await expect(page.getByRole('menuitem', { name: '运营概览' })).toBeVisible()

  await page.context().storageState({ path: authFile })
})
