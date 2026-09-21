import React from 'react';
import { BrowserRouter as Router, Routes, Route, Navigate, Outlet } from 'react-router-dom';
import Layout from './components/Layout';
import TicketListPage from './pages/TicketListPage';
import TicketDetailPage from './pages/TicketDetailPage';
import TicketCreatePage from './pages/TicketCreatePage';
import TicketEditPage from './pages/TicketEditPage';
import LoginPage from './pages/LoginPage';
import PhoneLinkPage from './pages/PhoneLinkPage';
import AssignPhonePage from './pages/AssignPhonePage';
import WhatsAppPage from './pages/WhatsAppPage';
import { authService } from './services/authService';

function RequireAuth() {
  return authService.isAuthenticated() ? <Outlet /> : <Navigate to="/login" replace />;
}

function App() {
  return (
    <Router>
      <Layout>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<RequireAuth />}>
            <Route path="/" element={<TicketListPage />} />
            <Route path="/ticket/new" element={<TicketCreatePage />} />
            <Route path="/ticket/:id" element={<TicketDetailPage />} />
            <Route path="/ticket/:id/edit" element={<TicketEditPage />} />
            <Route path="/ticket/:id/assign-phone" element={<AssignPhonePage />} />
            <Route path="/phone-link" element={<PhoneLinkPage />} />
            <Route path="/whatsapp" element={<WhatsAppPage />} />
          </Route>
        </Routes>
      </Layout>
    </Router>
  );
}

export default App;
