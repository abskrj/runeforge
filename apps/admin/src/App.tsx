import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import LoginPage from './pages/LoginPage'
import RegisterPage from './pages/RegisterPage'
import DashboardLayout from './components/DashboardLayout'
import OverviewPage from './pages/OverviewPage'
import APIKeysPage from './pages/APIKeysPage'
import TeamPage from './pages/TeamPage'
import BrandingPage from './pages/BrandingPage'
import UsagePage from './pages/UsagePage'
import EgressPage from './pages/EgressPage'
import SnippetsPage from './pages/SnippetsPage'
import SnippetEditorPage from './pages/SnippetEditorPage'
import ProtectedRoute from './components/ProtectedRoute'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/" element={<Navigate to="/dashboard/overview" replace />} />
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <DashboardLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<Navigate to="overview" replace />} />
          <Route path="overview" element={<OverviewPage />} />
          <Route path="api-keys" element={<APIKeysPage />} />
          <Route path="team" element={<TeamPage />} />
          <Route path="branding" element={<BrandingPage />} />
          <Route path="usage" element={<UsagePage />} />
          <Route path="egress" element={<EgressPage />} />
          <Route path="snippets" element={<SnippetsPage />} />
          <Route path="snippets/:id" element={<SnippetEditorPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
