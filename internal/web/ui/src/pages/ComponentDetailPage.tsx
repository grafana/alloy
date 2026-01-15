import { type FC, useEffect, useState } from 'react';
import { useParams } from 'react-router';

import { ComponentView } from '../features/component/ComponentView';
import { type ComponentDetail, type ComponentInfo, componentInfoByID } from '../features/component/types';
import { useComponentInfo } from '../hooks/componentInfo';
import { parseID } from '../utils/id';

const contentStyle: React.CSSProperties = {
  maxWidth: '1440px',
  marginLeft: 'auto',
  marginRight: 'auto',
  padding: '20px',
};

const messageStyle: React.CSSProperties = {
  color: '#666',
  fontStyle: 'italic',
  margin: '0 0 0.5rem 0',
};

const endpointStyle: React.CSSProperties = {
  fontSize: '0.875rem',
  fontFamily: 'monospace',
  color: '#888',
  margin: 0,
};

const ComponentDetailPage: FC = () => {
  const { '*': id } = useParams();
  const { moduleID } = parseID(id || '');
  const [components] = useComponentInfo(moduleID, false);
  const infoByID = componentInfoByID(components);

  const [component, setComponent] = useState<ComponentDetail | undefined>(undefined);
  const [loadingEndpoint, setLoadingEndpoint] = useState<string | null>(null);

  useEffect(
    function () {
      if (id === undefined) {
        return;
      }

      const fetchURL = `./api/v0/web/components/${id}`;
      setLoadingEndpoint(fetchURL);

      const worker = async () => {
        // Request is relative to the <base> tag inside of <head>.
        const resp = await fetch(fetchURL, {
          cache: 'no-cache',
          credentials: 'same-origin',
        });
        const data: ComponentDetail = await resp.json();

        for (const moduleID of data.createdModuleIDs || []) {
          const modulesURL = `./api/v0/web/modules/${moduleID}/components`;

          const moduleComponentsResp = await fetch(modulesURL, {
            cache: 'no-cache',
            credentials: 'same-origin',
          });
          const moduleComponents = (await moduleComponentsResp.json()) as ComponentInfo[];

          data.moduleInfo = (data.moduleInfo || []).concat(moduleComponents);
        }

        setComponent(data);
        setLoadingEndpoint(null);
      };

      worker().catch(console.error);
    },
    [id]
  );

  if (component) {
    return <ComponentView component={component} info={infoByID} />;
  }

  return (
    <main style={contentStyle}>
      <p style={messageStyle}>Loading component data...</p>
      {loadingEndpoint && <p style={endpointStyle}>Fetching: {loadingEndpoint}</p>}
    </main>
  );
};

export default ComponentDetailPage;
