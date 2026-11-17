import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AppShell } from './shell/AppShell';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Go RequestCoordinator owns retry/backoff/cooldown; never duplicate retries in TanStack Query (plan.md 6.1).
      retry: false,
      refetchOnWindowFocus: false,
    },
  },
});

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AppShell />
    </QueryClientProvider>
  );
}

export default App;
