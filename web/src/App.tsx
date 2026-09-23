import React, { useEffect } from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate, Outlet, useNavigate } from 'react-router-dom';
import { useAuth } from 'react-oidc-context';
import Layout from './components/Layout';
import HydraOAuth from './components/HydraOAuth';
import TicketListPage from './pages/TicketListPage';
import TicketDetailPage from './pages/TicketDetailPage';
import TicketCreatePage from './pages/TicketCreatePage';
import TicketEditPage from './pages/TicketEditPage';
import LoginPage from './pages/LoginPage';
import ConsentPage from './pages/ConsentPage';
import PhoneLinkPage from './pages/PhoneLinkPage';
import AssignPhonePage from './pages/AssignPhonePage';
import WhatsAppPage from './pages/WhatsAppPage';
import KanbanPage from './pages/KanbanPage';
import MessagesPage from './pages/MessagesPage';
import SettingsPage from './pages/SettingsPage';
import ReportsPage from './pages/ReportsPage';
import ExecutivePage from './pages/ExecutivePage';
import DashboardBuilderPage from './pages/DashboardBuilderPage';
import { authService } from './services/authService';

function ProtectedLayout() {
  const auth = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!auth) return;
    const handleExpired = () => navigate('/login', { replace: true });
    auth.events.addAccessTokenExpired(handleExpired);
    return () => auth.events.removeAccessTokenExpired(handleExpired);
  }, [auth, navigate]);

  // react-oidc-context sigue procesando el code/state de vuelta de Hydra: esperar,
  // si no, isAuthenticated todavía es false y rebota a /login antes de terminar.
  if (auth?.isLoading) {
    return <p className="muted" style={{ padding: 24 }}>Cargando…</p>;
  }

  const authenticated = authService.isAuthenticated() || Boolean(auth?.isAuthenticated);
  if (!authenticated) {
    return <Navigate to="/login" replace />;
  }
  return (
    <Layout>
      <Outlet />
    </Layout>
  );
}

function App() {
  return (
    <HydraOAuth>
      <Router>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/consent" element={<ConsentPage />} />
          <Route element={<ProtectedLayout />}>
            <Route path="/" element={<TicketListPage />} />
            <Route path="/kanban" element={<KanbanPage />} />
            <Route path="/reports" element={<ReportsPage />} />
            <Route path="/executive" element={<ExecutivePage />} />
            <Route path="/dashboard-builder" element={<DashboardBuilderPage />} />
            <Route path="/ticket/new" element={<TicketCreatePage />} />
            <Route path="/ticket/:id" element={<TicketDetailPage />} />
            <Route path="/ticket/:id/edit" element={<TicketEditPage />} />
            <Route path="/ticket/:id/assign-phone" element={<AssignPhonePage />} />
            <Route path="/phone-link" element={<PhoneLinkPage />} />
            <Route path="/whatsapp" element={<WhatsAppPage />} />
            <Route path="/messages" element={<MessagesPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
        </Routes>
      </Router>
    </HydraOAuth>
  );
}

export default App;
