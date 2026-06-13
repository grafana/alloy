import { BrowserRouter, Route, Routes } from 'react-router';

import Navbar from './features/layout/Navbar';
import PageClusteringPeers from './pages/Clustering';
import ComponentDetailPage from './pages/ComponentDetailPage';
import Graph from './pages/Graph';
import PageLiveDebugging from './pages/LiveDebugging';
import PageComponentList from './pages/PageComponentList';
import PageRemoteComponentList from './pages/PageRemoteComponentList';
import RemoteComponentDetailPage from './pages/RemoteComponentDetailPage';
import TargetsPage from './pages/TargetsPage';

interface Props {
  basePath: string;
}

const Router = ({ basePath }: Props) => {
  return (
    <BrowserRouter basename={basePath}>
      <Navbar />
      <main>
        <Routes>
          <Route path="/" element={<PageComponentList />} />
          <Route path="/remotecfg" element={<PageRemoteComponentList />} />

          <Route path="/component/*" element={<ComponentDetailPage />} />
          <Route path="/remotecfg/component/*" element={<RemoteComponentDetailPage />} />

          <Route path="/graph/*" element={<Graph />} />
          <Route path="/clustering" element={<PageClusteringPeers />} />
          <Route path="/targets" element={<TargetsPage />} />
          <Route path="/debug/*" element={<PageLiveDebugging />} />
        </Routes>
      </main>
    </BrowserRouter>
  );
};

export default Router;
