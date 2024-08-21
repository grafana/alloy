import { useLocation } from 'react-router-dom';
import { faCubes } from '@fortawesome/free-solid-svg-icons';

import ComponentList from '../features/component/ComponentList';
import { ComponentInfo, SortOrder } from '../features/component/types';
import Page from '../features/layout/Page';
import { useComponentInfo } from '../hooks/componentInfo';

const fieldMappings: { [key: string]: (comp: ComponentInfo) => string | undefined } = {
  Health: (comp) => comp.health?.state?.toString(),
  ID: (comp) => comp.localID,
  // Add new fields if needed here.
};

function getSortValue(component: ComponentInfo, field: string): string | undefined {
  const valueGetter = fieldMappings[field];
  return valueGetter ? valueGetter(component) : undefined;
}

function PageComponentList() {
  const location = useLocation();
  const useRemotecfg = location.pathname.startsWith('/remotecfg');
  const pageName = useRemotecfg ? 'Remote Configuration' : 'Components';
  const pageDesc = useRemotecfg ? 'List of remote configuration pipelines' : 'List of defined components';

  const [components, setComponents] = useComponentInfo('', useRemotecfg);

  // TODO: make this sorting logic reusable
  const handleSorting = (sortField: string, sortOrder: SortOrder): void => {
    if (!sortField || !sortOrder) return;
    const sorted = [...components].sort((a, b) => {
      const sortValueA = getSortValue(a, sortField);
      const sortValueB = getSortValue(b, sortField);
      if (!sortValueA) return 1;
      if (!sortValueB) return -1;
      return (
        sortValueA.localeCompare(sortValueB, 'en', {
          numeric: true,
        }) * (sortOrder === SortOrder.ASC ? 1 : -1)
      );
    });
    setComponents(sorted);
  };

  return (
    <Page name={pageName} desc={pageDesc} icon={faCubes}>
      <ComponentList components={components} useRemotecfg={useRemotecfg} handleSorting={handleSorting} />
    </Page>
  );
}

export default PageComponentList;
