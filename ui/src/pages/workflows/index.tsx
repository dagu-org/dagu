import { Search } from 'lucide-react';
import React from 'react';
import { useLocation } from 'react-router-dom';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { AppBarContext } from '../../contexts/AppBarContext';
import WorkflowTable from '../../features/workflows/components/workflow-list/WorkflowTable';
import { useQuery } from '../../hooks/api';
import LoadingIndicator from '../../ui/LoadingIndicator';
import Title from '../../ui/Title';

function Workflows() {
  const query = new URLSearchParams(useLocation().search);
  const appBarContext = React.useContext(AppBarContext);
  const [searchText, setSearchText] = React.useState(query.get('name') || '');
  const [apiSearchText, setAPISearchText] = React.useState(
    query.get('name') || ''
  );

  React.useEffect(() => {
    appBarContext.setTitle('Workflows');
  }, [appBarContext]);

  const { data, isLoading } = useQuery(
    '/workflows',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
          name: apiSearchText ? apiSearchText : undefined,
        },
      },
    },
    {
      // This ensures the query only runs when apiSearchText changes
      // which is controlled by the search button or Enter key
      revalidateIfStale: true,
      revalidateOnFocus: true,
      revalidateOnReconnect: true,
    }
  );

  const addSearchParam = (key: string, value: string) => {
    const locationQuery = new URLSearchParams(window.location.search);
    if (value) {
      locationQuery.set(key, value);
    } else {
      locationQuery.delete(key);
    }
    window.history.pushState(
      {},
      '',
      `${window.location.pathname}?${locationQuery.toString()}`
    );
  };

  const handleSearch = () => {
    setAPISearchText(searchText);
    addSearchParam('name', searchText);
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchText(e.target.value);
  };

  const handleInputKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handleSearch();
    }
  };

  return (
    <div className="flex flex-col">
      <Title>Workflows</Title>
      <div className="flex items-center gap-2 mb-4">
        <Input
          placeholder="Filter by workflow name..."
          value={searchText}
          onChange={handleInputChange}
          onKeyPress={handleInputKeyPress}
          className="max-w-sm"
        />
        <Button onClick={handleSearch}>
          <Search size={18} className="mr-2" />
          Search
        </Button>
      </div>
      {isLoading ? (
        <LoadingIndicator />
      ) : (
        <WorkflowTable workflows={data?.workflows || []} />
      )}
    </div>
  );
}

export default Workflows;
