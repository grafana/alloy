import { type FC, useEffect, useState } from 'react';
import { useParams } from 'react-router';

import { ComponentView } from '../features/component/ComponentView';
import { type ComponentDetail, type ComponentInfo, componentInfoByID } from '../features/component/types';
import { useComponentInfo } from '../hooks/componentInfo';
import { parseID } from '../utils/id';

const ComponentDetailPage: FC = () => {
  const { '*': id } = useParams();
  const { moduleID } = parseID(id || '');
  const [components] = useComponentInfo(moduleID, false);
  const infoByID = componentInfoByID(components);

  const [component, setComponent] = useState<ComponentDetail | undefined>(undefined);

  useEffect(
    function () {
      if (id === undefined) {
        return;
      }

      const fetchURL = `./api/v0/web/components/${id}`;
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
      };

      worker().catch(console.error);
    },
    [id]
  );

  return component ? <ComponentView component={component} info={infoByID} /> : <div></div>;
};

export default ComponentDetailPage;
