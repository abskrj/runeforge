import { BrowserRouter, Routes, Route, Navigate, useSearchParams } from 'react-router-dom'
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
import EmbedPage from './pages/EmbedPage'
import EmbedEntryPage from './pages/EmbedEntryPage'
import VariablesPage from './pages/VariablesPage'
import LibrariesPage from './pages/LibrariesPage'
import LibraryEditorPage from './pages/LibraryEditorPage'
import ProtectedRoute from './components/ProtectedRoute'

function RootRedirect() {
  const [params] = useSearchParams()
  if (params.get('token')?.startsWith('et_')) {
    return <EmbedEntryPage />
  }
  return <Navigate to="/dashboard/overview" replace />
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<RootRedirect />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
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
          <Route path="libraries" element={<LibrariesPage />} />
          <Route path="libraries/:id" element={<LibraryEditorPage />} />
          <Route path="variables" element={<VariablesPage />} />
          <Route path="embed" element={<EmbedPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
