import { Activity, Gauge, MessageCircle, Play, ShieldAlert, Ticket } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';

type Mode = 'proxy' | 'provider';

type Comment = {
  id: string;
  body: string;
  createdAt: string;
};

type SupportTicket = {
  id: string;
  subject: string;
  status: string;
  comments: Comment[];
};

type Counters = {
  allowed: number;
  blocked: number;
  errors: number;
};

const env = import.meta.env as Record<string, string | undefined>;
const mode: Mode = env.VITE_METERGATE_MODE === 'provider' ? 'provider' : 'proxy';
const endpoint =
  mode === 'proxy'
    ? (env.VITE_PROXY_GRAPHQL_URL ?? 'http://localhost:3000/graphql')
    : (env.VITE_PROVIDER_GRAPHQL_URL ?? 'http://localhost:8000/graphql');
const apiKey = env.VITE_DEMO_API_KEY ?? 'mg_demo_local_plaintext_key';

async function graphql<T>(query: string, variables: Record<string, unknown> = {}): Promise<{ data?: T; response: Response; body: unknown }> {
  const response = await fetch(endpoint, {
    method: 'POST',
    headers: {
      'content-type': 'application/json',
      'x-api-key': apiKey,
      'x-request-id': crypto.randomUUID()
    },
    body: JSON.stringify({ query, variables })
  });
  const body = (await response.json().catch(() => null)) as unknown;
  return { data: response.ok ? (body as { data: T }).data : undefined, response, body };
}

export function App() {
  const [tickets, setTickets] = useState<SupportTicket[]>([]);
  const [selectedId, setSelectedId] = useState('TCK-1001');
  const [comment, setComment] = useState('');
  const [fakeCount, setFakeCount] = useState(8);
  const [counters, setCounters] = useState<Counters>({ allowed: 0, blocked: 0, errors: 0 });
  const [lastRateLimit, setLastRateLimit] = useState('No requests yet');
  const [busy, setBusy] = useState(false);

  const selected = useMemo<SupportTicket | undefined>(
    () => tickets.find((ticket) => ticket.id === selectedId) ?? tickets.at(0),
    [tickets, selectedId]
  );

  async function loadTickets() {
    const result = await graphql<{ tickets: SupportTicket[] }>('query Tickets { tickets { id subject status comments { id body createdAt } } }');
    if (result.data?.tickets) {
      const loadedTickets = result.data.tickets;
      setTickets(loadedTickets);
      setSelectedId((current) => current || loadedTickets.at(0)?.id || '');
    }
    updateRateLimit(result.response, result.body);
  }

  async function addComment() {
    if (!selected || comment.trim() === '') {
      return;
    }
    const result = await graphql<{ addComment: SupportTicket }>(
      'mutation AddComment($ticketId: ID!, $body: String!) { addComment(ticketId: $ticketId, body: $body) { id subject status comments { id body createdAt } } }',
      { ticketId: selected.id, body: comment.trim() }
    );
    if (result.data?.addComment) {
      setComment('');
      await loadTickets();
    }
    updateRateLimit(result.response, result.body);
  }

  async function generateFakeRequests() {
    setBusy(true);
    const nextCounters: Counters = { allowed: 0, blocked: 0, errors: 0 };
    try {
      for (let index = 0; index < fakeCount; index += 1) {
        const result = await graphql<{ tickets: SupportTicket[] }>('query Tickets { tickets { id subject status } }');
        if (result.response.status === 429 || result.response.status === 401 || result.response.status === 403) {
          nextCounters.blocked += 1;
        } else if (result.response.ok) {
          nextCounters.allowed += 1;
        } else {
          nextCounters.errors += 1;
        }
        updateRateLimit(result.response, result.body);
      }
      setCounters((current) => ({
        allowed: current.allowed + nextCounters.allowed,
        blocked: current.blocked + nextCounters.blocked,
        errors: current.errors + nextCounters.errors
      }));
    } finally {
      setBusy(false);
    }
  }

  function updateRateLimit(response: Response, body: unknown) {
    const limit = response.headers.get('x-ratelimit-limit');
    const remaining = response.headers.get('x-ratelimit-remaining');
    const reset = response.headers.get('x-ratelimit-reset');
    const reason = typeof body === 'object' && body !== null && 'reason' in body ? String((body as { reason: unknown }).reason) : response.headers.get('x-metergate-decision');
    setLastRateLimit(`${reason ?? 'ALLOWED'} | limit ${limit ?? '-'} | remaining ${remaining ?? '-'} | reset ${reset ?? '-'}s`);
  }

  useEffect(() => {
    void loadTickets();
  }, []);

  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <Gauge aria-hidden="true" />
          <div>
            <h1>MeterGate</h1>
            <p>Support tickets demo</p>
          </div>
        </div>
        <div className="mode-pill">
          <Activity aria-hidden="true" />
          <span>{mode}</span>
        </div>
        <nav className="ticket-list" aria-label="Tickets">
          {tickets.map((ticket) => (
            <button
              key={ticket.id}
              className={ticket.id === selected?.id ? 'ticket-row active' : 'ticket-row'}
              onClick={() => {
                setSelectedId(ticket.id);
              }}
            >
              <Ticket aria-hidden="true" />
              <span>
                <strong>{ticket.id}</strong>
                {ticket.subject}
              </span>
            </button>
          ))}
        </nav>
      </aside>

      <section className="workspace">
        <header className="toolbar">
          <div>
            <h2>{selected?.subject ?? 'Loading tickets'}</h2>
            <p>{selected?.status ?? 'pending'}</p>
          </div>
          <div className="counters" aria-label="Request counters">
            <span>{counters.allowed} allowed</span>
            <span>{counters.blocked} blocked</span>
            <span>{counters.errors} errors</span>
          </div>
        </header>

        <section className="request-panel">
          <label>
            <span>Fake requests</span>
            <input
              min={1}
              max={50}
              type="number"
              value={fakeCount}
              onChange={(event) => {
                setFakeCount(Number(event.target.value));
              }}
            />
          </label>
          <button
            className="primary-button"
            onClick={() => {
              void generateFakeRequests();
            }}
            disabled={busy}
          >
            <Play aria-hidden="true" />
            {busy ? 'Running' : 'Generate'}
          </button>
          <output>
            <ShieldAlert aria-hidden="true" />
            {lastRateLimit}
          </output>
        </section>

        <section className="detail">
          <div className="comments">
            {selected?.comments.map((item) => (
              <article key={item.id} className="comment">
                <MessageCircle aria-hidden="true" />
                <div>
                  <p>{item.body}</p>
                  <time>{item.createdAt}</time>
                </div>
              </article>
            ))}
          </div>
          <div className="composer">
            <textarea
              value={comment}
              onChange={(event) => {
                setComment(event.target.value);
              }}
              placeholder="Add a support note"
            />
            <button
              onClick={() => {
                void addComment();
              }}
              disabled={!selected || comment.trim() === ''}
            >
              Add comment
            </button>
          </div>
        </section>
      </section>
    </main>
  );
}
