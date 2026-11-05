import {
  AppstoreOutlined,
  AuditOutlined,
  CloudUploadOutlined,
  CreditCardOutlined,
  CrownOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  ExperimentOutlined,
  GiftOutlined,
  LogoutOutlined,
  PieChartOutlined,
  PlayCircleOutlined,
  TeamOutlined,
} from '@ant-design/icons'
import { Button, Layout, Menu, Spin, Typography } from 'antd'
import { useEffect, useState } from 'react'
import {
  Navigate,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from 'react-router-dom'
import { api } from './api'
import { AuthContext } from './auth'
import type { AdminPrincipal } from './types'
import AuditPage from './pages/AuditPage'
import CalibrationPage from './pages/CalibrationPage'
import CoveragePage from './pages/CoveragePage'
import DashboardPage from './pages/DashboardPage'
import FamiliesPage from './pages/FamiliesPage'
import ImportPage from './pages/ImportPage'
import Login from './pages/Login'
import MembershipOrdersPage from './pages/MembershipOrdersPage'
import MembershipPlansPage from './pages/MembershipPlansPage'
import QuestionEdit from './pages/QuestionEdit'
import Questions from './pages/Questions'
import QuizPreviewPage from './pages/QuizPreviewPage'
import TopicsPage from './pages/TopicsPage'

const { Header, Sider, Content } = Layout

export default function App() {
  const [principal, setPrincipal] = useState<AdminPrincipal | null>(null)
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()
  const location = useLocation()

  useEffect(() => {
    void (async () => {
      try {
        setPrincipal(await api.me())
      } catch {
        // 令牌可能已过期或不存在（401），视为未登录。
        setPrincipal(null)
      } finally {
        setLoading(false)
      }
    })()
  }, [])

  if (loading) {
    return (
      <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center' }}>
        <Spin size="large" />
      </div>
    )
  }

  if (!principal) {
    return <Login onLogin={setPrincipal} />
  }

  const can = (permission: string) =>
    Boolean(
      principal.permissions?.includes('*') ||
        principal.permissions?.includes(permission),
    )

  const selected = location.pathname.startsWith('/questions')
    ? '/questions'
    : location.pathname.startsWith('/topics')
      ? '/topics'
      : location.pathname

  return (
    <AuthContext.Provider value={{ principal, can }}>
      <Layout style={{ minHeight: '100vh' }}>
        <Sider theme="light" width={200}>
          <div style={{ padding: 20 }}>
            <Typography.Title level={4} style={{ margin: 0 }}>
              OpenQuizGo
            </Typography.Title>
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              题库治理后台
            </Typography.Text>
          </div>
          <Menu
            mode="inline"
            selectedKeys={[selected]}
            onClick={({ key }) => navigate(key)}
            items={[
              {
                key: '/dashboard',
                icon: <DashboardOutlined />,
                label: '运营概览',
                disabled: !can('dashboard.read'),
              },
              { key: '/questions', icon: <DatabaseOutlined />, label: '题目管理' },
              {
                key: '/import',
                icon: <CloudUploadOutlined />,
                label: '批量导入',
                disabled: !can('question.import'),
              },
              {
                key: '/topics',
                icon: <GiftOutlined />,
                label: '专题题包',
              },
              {
                key: '/families',
                icon: <TeamOutlined />,
                label: '家庭设备',
                disabled: !can('family.read'),
              },
              {
                key: '/membership',
                icon: <CrownOutlined />,
                label: '会员套餐',
                disabled: !can('membership.read'),
              },
              {
                key: '/membership/orders',
                icon: <CreditCardOutlined />,
                label: '会员订单',
                disabled: !can('membership.read'),
              },
              {
                key: '/quiz-preview',
                icon: <PlayCircleOutlined />,
                label: '组卷预览',
              },
              {
                key: '/coverage',
                icon: <PieChartOutlined />,
                label: '覆盖率',
              },
              {
                key: '/calibration',
                icon: <ExperimentOutlined />,
                label: '难度校准',
              },
              {
                key: '/audit',
                icon: <AuditOutlined />,
                label: '审计日志',
                disabled: !can('audit.read'),
              },
            ]}
          />
        </Sider>
        <Layout>
          <Header
            style={{
              background: '#fff',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              paddingInline: 24,
            }}
          >
            <Typography.Text>
              <AppstoreOutlined /> 题库治理
            </Typography.Text>
            <span>
              <Typography.Text style={{ marginRight: 16 }}>
                {principal.display_name || principal.username}
              </Typography.Text>
              <Button
                icon={<LogoutOutlined />}
                onClick={() => {
                  void (async () => {
                    try {
                      await api.logout()
                    } catch {
                      // 网络失败也按退出处理，Cookie 可由服务端过期或后续登录覆盖。
                    } finally {
                      setPrincipal(null)
                    }
                  })()
                }}
              >
                退出
              </Button>
            </span>
          </Header>
          <Content style={{ padding: 24, background: '#f5f7f6' }}>
            <Routes>
              <Route path="/" element={<Navigate to="/questions" replace />} />
              <Route path="/dashboard" element={<DashboardPage />} />
              <Route path="/questions" element={<Questions />} />
              <Route path="/questions/new" element={<QuestionEdit />} />
              <Route path="/questions/:id" element={<QuestionEdit />} />
              <Route path="/topics" element={<TopicsPage />} />
              <Route path="/quiz-preview" element={<QuizPreviewPage />} />
              <Route path="/families" element={<FamiliesPage />} />
              <Route path="/membership" element={<MembershipPlansPage />} />
              <Route path="/membership/orders" element={<MembershipOrdersPage />} />
              <Route path="/import" element={<ImportPage />} />
              <Route path="/audit" element={<AuditPage />} />
              <Route path="/coverage" element={<CoveragePage />} />
              <Route path="/calibration" element={<CalibrationPage />} />
            </Routes>
          </Content>
        </Layout>
      </Layout>
    </AuthContext.Provider>
  )
}
