import { Component, type ErrorInfo, type ReactNode } from 'react';
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

interface BoundaryState {
  error: Error | null;
}

class AppErrorBoundary extends Component<{ children: ReactNode }, BoundaryState> {
  state: BoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): BoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('LD Studio UI error:', error, info);
  }

  render() {
    if (this.state.error) {
      return (
        <div
          style={{
            minHeight: '100vh',
            padding: 24,
            background: '#EDEAE3',
            color: '#292623',
            fontFamily: 'system-ui, -apple-system, Segoe UI, sans-serif',
          }}
        >
          <h2 style={{ margin: '0 0 8px', fontSize: 16 }}>界面渲染发生异常</h2>
          <p style={{ margin: '0 0 12px', fontSize: 13, color: '#615A52' }}>
            {this.state.error.message || '未知界面错误'}
          </p>
          <button
            type="button"
            onClick={() => this.setState({ error: null })}
            style={{
              padding: '6px 12px',
              borderRadius: 6,
              border: '1px solid #C8BFB2',
              background: '#FAF9F5',
              cursor: 'pointer',
            }}
          >
            重试渲染
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}

export function App() {
  return (
    <AppErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <AppShell />
      </QueryClientProvider>
    </AppErrorBoundary>
  );
}

export default App;
