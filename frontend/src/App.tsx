import {
  useCallback,
  useEffect,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import {
  Link,
  NavLink,
  Route,
  Routes,
  useNavigate,
  useParams,
} from "react-router-dom";
import {
  api,
  post,
  APIError,
  timestamp,
  type Project,
  type Page,
  type Endpoint,
  type Event,
  type Delivery,
  type Key,
} from "./api";

function useLoad<T>(path: string) {
  const [data, setData] = useState<T>();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [version, setVersion] = useState(0);
  const reload = useCallback(() => setVersion((v) => v + 1), []);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    api<T>(path, { signal: controller.signal })
      .then(setData)
      .catch((e) => {
        if (e.name !== "AbortError") setError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [path, version]);
  return { data, error, loading, reload };
}
function ErrorBox({ error }: { error: string }) {
  return error ? (
    <div role="alert" className="error">
      {error}
    </div>
  ) : null;
}
function State({
  loading,
  error,
  empty = false,
  children,
}: {
  loading: boolean;
  error: string;
  empty?: boolean;
  children: ReactNode;
}) {
  if (loading) return <div className="state">Loading records…</div>;
  if (error) return <ErrorBox error={error} />;
  if (empty)
    return (
      <div className="state">
        <strong>No records yet</strong>
        <p>Create a project, add a destination, and ingest your first event.</p>
      </div>
    );
  return children;
}
function Badge({ status }: { status: string }) {
  return <span className={`badge ${status}`}>{status}</span>;
}
function Heading({
  title,
  description,
  children,
}: {
  title: string;
  description: string;
  children?: ReactNode;
}) {
  return (
    <header className="page-heading">
      <div>
        <p className="eyebrow">WORKSPACE / OPERATIONS</p>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {children}
    </header>
  );
}
function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
    </label>
  );
}
function Pager({
  offset,
  count,
  onChange,
}: {
  offset: number;
  count: number;
  onChange: (n: number) => void;
}) {
  return (
    <div className="pager">
      <span>
        Showing {count ? offset + 1 : 0}–{offset + count}
      </span>
      <button
        disabled={offset === 0}
        onClick={() => onChange(Math.max(0, offset - 25))}
      >
        Previous
      </button>
      <button disabled={count < 25} onClick={() => onChange(offset + 25)}>
        Next
      </button>
    </div>
  );
}

export function App() {
  const [auth, setAuth] = useState<boolean | null>(null);
  const [error, setError] = useState("");
  const check = useCallback(() => {
    setError("");
    api("/api/auth/me")
      .then(() => setAuth(true))
      .catch((e) => {
        if (e instanceof APIError && e.status === 401) setAuth(false);
        else setError(e.message);
      });
  }, []);
  useEffect(check, [check]);
  if (error)
    return (
      <main className="auth">
        <ErrorBox error={error} />
        <button onClick={check}>Reconnect</button>
      </main>
    );
  if (auth === null)
    return <main className="auth">Connecting to PulseRoute…</main>;
  if (!auth) return <Login onDone={() => setAuth(true)} />;
  return (
    <div className="shell">
      <aside>
        <Link className="brand" to="/">
          <span className="brand-mark">P</span>PulseRoute
        </Link>
        <div className="workspace-label">DELIVERY CONTROL</div>
        <nav>
          {[
            ["/", "Overview"],
            ["/projects", "Projects"],
            ["/endpoints", "Endpoints"],
            ["/events", "Events"],
            ["/deliveries", "Deliveries"],
            ["/dead-letters", "Dead letters"],
            ["/settings", "API keys"],
          ].map(([to, label]) => (
            <NavLink key={to} to={to} end={to === "/"}>
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <span className="live-dot" /> Synthetic data workspace
          <button
            onClick={async () => {
              try {
                await post("/api/auth/logout", {});
                setAuth(false);
              } catch (e) {
                setError((e as Error).message);
              }
            }}
          >
            Sign out
          </button>
        </div>
      </aside>
      <main className="content">
        <div className="topbar">
          <span>Webhook reliability platform</span>
          <span className="version">PulseRoute / v1</span>
        </div>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/projects" element={<Projects />} />
          <Route path="/projects/:id" element={<ProjectDetails />} />
          <Route path="/endpoints" element={<EndpointPage />} />
          <Route path="/settings" element={<Keys />} />
          <Route path="/events" element={<Records kind="events" />} />
          <Route path="/events/:id" element={<EventDetails />} />
          <Route path="/deliveries" element={<Records kind="deliveries" />} />
          <Route path="/deliveries/:id" element={<DeliveryDetails />} />
          <Route
            path="/dead-letters"
            element={<Records kind="dead-letters" />}
          />
          <Route
            path="*"
            element={
              <Heading
                title="Page not found"
                description="Choose a section from the navigation."
              />
            }
          />
        </Routes>
      </main>
    </div>
  );
}
function Login({ onDone }: { onDone: () => void }) {
  const [register, setRegister] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const f = new FormData(e.currentTarget);
    try {
      await post(
        `/api/auth/${register ? "register" : "login"}`,
        Object.fromEntries(f),
      );
      onDone();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className="auth">
      <div className="auth-story">
        <Link className="brand" to="/">
          <span className="brand-mark">P</span>PulseRoute
        </Link>
        <p className="eyebrow">DELIVERY, WITH A PAPER TRAIL</p>
        <h1>
          Every event.
          <br />
          Every attempt.
          <br />
          Accounted for.
        </h1>
        <p>
          Inspect webhook traffic, recover failed deliveries, and understand
          what happened between send and receive.
        </p>
        <div className="route-line">
          <span>Event</span>
          <b>→</b>
          <span>Queue</span>
          <b>→</b>
          <span>Destination</span>
        </div>
      </div>
      <section className="auth-panel">
        <p className="eyebrow">YOUR OPERATIONS WORKSPACE</p>
        <h2>{register ? "Create your account" : "Welcome back"}</h2>
        <p>Use synthetic data in this portfolio demo.</p>
        <form onSubmit={submit}>
          <Field label="Email">
            <input name="email" type="email" required autoComplete="email" />
          </Field>
          <Field label="Password">
            <input
              name="password"
              type="password"
              minLength={12}
              maxLength={72}
              required
              autoComplete={register ? "new-password" : "current-password"}
            />
          </Field>
          <ErrorBox error={error} />
          <button className="primary" disabled={busy}>
            {busy ? "Please wait…" : register ? "Create account" : "Sign in"}
          </button>
        </form>
        <button
          className="text-button"
          onClick={() => {
            setRegister(!register);
            setError("");
          }}
        >
          {register
            ? "Already have an account? Sign in"
            : "New here? Create an account"}
        </button>
      </section>
    </main>
  );
}

function Dashboard() {
  const metrics = useLoad<Record<string, number>>("/api/dashboard");
  const events = useLoad<Page<Event>>("/api/events?limit=5");
  const deliveries = useLoad<Page<Delivery>>("/api/deliveries?limit=5");
  return (
    <>
      <Heading
        title="Delivery overview"
        description="A live view of your events and their delivery outcomes."
      >
        <button
          onClick={() => {
            metrics.reload();
            events.reload();
            deliveries.reload();
          }}
        >
          Refresh
        </button>
      </Heading>
      <ErrorBox error={metrics.error} />
      <div className="metrics">
        {[
          ["events_today", "Events today"],
          ["succeeded", "Delivered"],
          ["retrying", "Retrying"],
          ["dead", "Dead letters"],
          ["success_rate", "Success rate"],
          ["average_latency_ms", "Avg. attempt latency"],
        ].map(([key, label]) => (
          <section className="metric" key={key}>
            <span>{label}</span>
            <strong>
              {metrics.loading || metrics.error
                ? "—"
                : `${(metrics.data?.[key] ?? 0).toLocaleString(undefined, { maximumFractionDigits: 1 })}${key === "success_rate" ? "%" : key === "average_latency_ms" ? " ms" : ""}`}
            </strong>
            <small>
              {key === "events_today"
                ? "Since midnight UTC"
                : key === "success_rate"
                  ? "Successful / terminal deliveries"
                  : "All-time activity"}
            </small>
          </section>
        ))}
      </div>
      <div className="notice">
        {metrics.data && !metrics.error && (
          <p>
            {metrics.data.events_received} events received · {metrics.data.deliveries_attempted} attempts · {metrics.data.failed} failed attempts
          </p>
        )}
        At-least-once delivery · Metrics refresh on demand; totals may be cached
        for 5 seconds.
      </div>
      <section className="panel">
        <div className="panel-title">
          <h2>Recent events</h2>
          <Link to="/events">View all →</Link>
        </div>
        <State {...events} empty={!events.data?.items.length}>
          <EventTable items={events.data?.items ?? []} />
        </State>
      </section>
      <section className="panel">
        <div className="panel-title">
          <h2>Recent deliveries</h2>
          <Link to="/deliveries">View all →</Link>
        </div>
        <State {...deliveries} empty={!deliveries.data?.items.length}>
          <DeliveryTable items={deliveries.data?.items ?? []} />
        </State>
      </section>
    </>
  );
}
function Projects() {
  const list = useLoad<Page<Project>>("/api/projects?limit=100");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const navigate = useNavigate();
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    try {
      const p = await post<Project>(
        "/api/projects",
        Object.fromEntries(new FormData(e.currentTarget)),
      );
      navigate(`/projects/${p.id}`);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <Heading
        title="Projects"
        description="Keep destinations, credentials, and events grouped by application."
      />
      <form className="inline-form panel" onSubmit={submit}>
        <Field label="Project name">
          <input
            name="name"
            required
            maxLength={100}
            placeholder="Order operations"
          />
        </Field>
        <button className="primary" disabled={busy}>
          Create project
        </button>
        <ErrorBox error={error} />
      </form>
      <State {...list} empty={!list.data?.items.length}>
        <div className="project-grid">
          {list.data?.items.map((p) => (
            <Link className="project-card" key={p.id} to={`/projects/${p.id}`}>
              <span className="project-icon">↗</span>
              <h2>{p.name}</h2>
              <code>{p.id.slice(0, 20)}…</code>
              <p>Created {timestamp(p.created_at)}</p>
              <span>Manage project →</span>
            </Link>
          ))}
        </div>
      </State>
    </>
  );
}
function ProjectDetails() {
  const { id = "" } = useParams();
  const p = useLoad<Project>(`/api/projects/${id}`);
  return (
    <>
      <Heading
        title={p.data?.name ?? "Project details"}
        description="Configure where events go and how failures are retried."
      />
      <ErrorBox error={p.error} />
      <EndpointManager project={id} />
      <section className="panel">
        <h2>Next: send an event</h2>
        <p>
          Create a key in <Link to="/settings">API keys</Link>, then send an
          authenticated request to <code>POST /v1/events</code>. Use the event
          envelope shown in the README.
        </p>
        <Link to={`/events?project_id=${id}`}>Inspect events →</Link>
      </section>
    </>
  );
}
function ProjectSelect({
  value,
  onChange,
}: {
  value: string;
  onChange: (s: string) => void;
}) {
  const p = useLoad<Page<Project>>("/api/projects?limit=100");
  return (
    <>
      <Field label="Project">
        <select value={value} onChange={(e) => onChange(e.target.value)}>
          <option value="">Select a project</option>
          {p.data?.items.map((p) => (
            <option value={p.id} key={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </Field>
      <ErrorBox error={p.error} />
    </>
  );
}
function EndpointPage() {
  const [project, setProject] = useState("");
  return (
    <>
      <Heading
        title="Webhook endpoints"
        description="Destination URLs, signing secrets, and retry policies."
      />
      <ProjectSelect value={project} onChange={setProject} />
      {project ? (
        <EndpointManager key={project} project={project} />
      ) : (
        <div className="state">Select a project to manage its endpoints.</div>
      )}
    </>
  );
}
function EndpointManager({ project }: { project: string }) {
  const list = useLoad<Page<Endpoint>>(
    `/api/projects/${project}/endpoints?limit=100`,
  );
  const [error, setError] = useState("");
  const [secret, setSecret] = useState("");
  const [editing, setEditing] = useState<Endpoint>();
  const [busy, setBusy] = useState(false);
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const form = e.currentTarget;
    const f = Object.fromEntries(new FormData(form));
    const body = {
      ...f,
      max_attempts: Number(f.max_attempts),
      base_delay_seconds: Number(f.base_delay_seconds),
      rate_per_second: Number(f.rate_per_second),
    };
    try {
      if (editing) {
        await api(`/api/endpoints/${editing.id}`, {
          method: "PATCH",
          body: JSON.stringify(body),
        });
        setEditing(undefined);
      } else {
        const out = await post<{ secret: string }>(
          `/api/projects/${project}/endpoints`,
          body,
        );
        setSecret(out.secret);
        form.reset();
      }
      list.reload();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function mutate(ep: Endpoint, remove = false) {
    if (
      remove &&
      !confirm(
        "Delete this endpoint? Pending work will be cancelled when claimed.",
      )
    )
      return;
    try {
      await api(`/api/endpoints/${ep.id}`, {
        method: remove ? "DELETE" : "PATCH",
        body: remove ? undefined : JSON.stringify({ active: !ep.active }),
      });
      list.reload();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <>
      <section className="panel">
        <h2>{editing ? "Edit endpoint" : "Add endpoint"}</h2>
        <form
          key={editing?.id ?? "new"}
          className="form-grid"
          onSubmit={submit}
        >
          <Field label="Name">
            <input
              name="name"
              defaultValue={editing?.name}
              required
              maxLength={100}
            />
          </Field>
          <Field label="Destination URL">
            <input
              name="url"
              type="url"
              defaultValue={editing?.url}
              required
              placeholder="https://your-receiver.test/webhook"
            />
          </Field>
          <Field
            label={
              editing
                ? "New secret (blank keeps current)"
                : "Signing secret (blank generates one)"
            }
          >
            <input
              name="secret"
              type="password"
              minLength={16}
              maxLength={256}
              autoComplete="new-password"
            />
          </Field>
          <Field label="Maximum attempts">
            <input
              name="max_attempts"
              type="number"
              min={1}
              max={10}
              defaultValue={editing?.max_attempts ?? 5}
              required
            />
          </Field>
          <Field label="Base delay (seconds)">
            <input
              name="base_delay_seconds"
              type="number"
              min={1}
              max={3600}
              defaultValue={editing?.base_delay_seconds ?? 5}
              required
            />
          </Field>
          <Field label="Requests per second">
            <input
              name="rate_per_second"
              type="number"
              min={1}
              max={1000}
              defaultValue={editing?.rate_per_second ?? 10}
              required
            />
          </Field>
          <div>
            <button className="primary" disabled={busy}>
              {editing ? "Save endpoint" : "Add endpoint"}
            </button>
            {editing && (
              <button type="button" onClick={() => setEditing(undefined)}>
                Cancel
              </button>
            )}
          </div>
        </form>
        <ErrorBox error={error} />
        {secret && (
          <div className="secret">
            <strong>Save this signing secret; it is shown once.</strong>
            <code>{secret}</code>
            <button onClick={() => setSecret("")}>Dismiss</button>
          </div>
        )}
      </section>
      <section className="panel">
        <h2>Registered destinations</h2>
        <State {...list} empty={!list.data?.items.length}>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>Name / URL</th>
                  <th>Status</th>
                  <th>Policy</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {list.data?.items.map((ep) => (
                  <tr key={ep.id}>
                    <td>
                      <strong>{ep.name}</strong>
                      <small>{ep.url}</small>
                    </td>
                    <td>
                      <Badge status={ep.active ? "active" : "paused"} />
                    </td>
                    <td>
                      {ep.max_attempts} attempts · {ep.rate_per_second}/s
                    </td>
                    <td className="actions">
                      <button onClick={() => setEditing(ep)}>Edit</button>
                      <button onClick={() => mutate(ep)}>
                        {ep.active ? "Pause" : "Enable"}
                      </button>
                      <button onClick={() => mutate(ep, true)}>Delete</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </State>
      </section>
    </>
  );
}
function Keys() {
  const [project, setProject] = useState("");
  return (
    <>
      <Heading
        title="API keys"
        description="Project-scoped credentials for event ingestion. Keys are shown only at creation."
      />
      <ProjectSelect value={project} onChange={setProject} />
      {project && <KeyManager key={project} project={project} />}
    </>
  );
}
function KeyManager({ project }: { project: string }) {
  const list = useLoad<Page<Key>>(
    `/api/projects/${project}/api-keys?limit=100`,
  );
  const [key, setKey] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    try {
      const out = await post<{ key: string }>(
        `/api/projects/${project}/api-keys`,
        Object.fromEntries(new FormData(e.currentTarget)),
      );
      setKey(out.key);
      list.reload();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="panel">
      <form className="inline-form" onSubmit={submit}>
        <Field label="Key name">
          <input name="name" required maxLength={100} />
        </Field>
        <button className="primary" disabled={busy}>
          Create API key
        </button>
      </form>
      <ErrorBox error={error} />
      {key && (
        <div className="secret">
          <strong>Copy and store this key now.</strong>
          <code>{key}</code>
          <button onClick={() => setKey("")}>Dismiss</button>
        </div>
      )}
      <State {...list} empty={!list.data?.items.length}>
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Prefix</th>
              <th>Status</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {list.data?.items.map((k) => (
              <tr key={k.id}>
                <td>{k.name}</td>
                <td>
                  <code>{k.prefix}…</code>
                </td>
                <td>{k.revoked_at ? "Revoked" : "Active"}</td>
                <td>
                  <button
                    disabled={!!k.revoked_at}
                    onClick={async () => {
                      if (!confirm("Revoke this API key?")) return;
                      try {
                        await api(`/api/api-keys/${k.id}`, {
                          method: "DELETE",
                        });
                        list.reload();
                      } catch (e) {
                        setError((e as Error).message);
                      }
                    }}
                  >
                    Revoke
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </State>
    </section>
  );
}

function EventTable({ items }: { items: Event[] }) {
  return (
    <div className="table-scroll">
      <table>
        <thead>
          <tr>
            <th>Event ID</th>
            <th>Type</th>
            <th>Received</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {items.map((e) => (
            <tr key={e.id}>
              <td>
                <code>{e.event_id}</code>
              </td>
              <td>{e.type}</td>
              <td>{timestamp(e.created_at)}</td>
              <td>
                <Link to={`/events/${e.id}`}>Inspect →</Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
function DeliveryTable({ items }: { items: Delivery[] }) {
  return (
    <div className="table-scroll">
      <table>
        <thead>
          <tr>
            <th>Event / destination</th>
            <th>Status</th>
            <th>Attempts</th>
            <th>Created</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {items.map((d) => (
            <tr key={d.id}>
              <td>
                <strong>{d.external_event_id ?? d.event_id}</strong>
                <small>{d.endpoint_name}</small>
              </td>
              <td>
                <Badge status={d.status} />
              </td>
              <td>{d.attempt_count}</td>
              <td>{timestamp(d.created_at)}</td>
              <td>
                <Link to={`/deliveries/${d.id}`}>Inspect →</Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
function Records({ kind }: { kind: "events" | "deliveries" | "dead-letters" }) {
  const [project, setProject] = useState("");
  const [type, setType] = useState("");
  const [status, setStatus] = useState("");
  const [endpoint, setEndpoint] = useState("");
  const [date, setDate] = useState("");
  const [offset, setOffset] = useState(0);
  const params = new URLSearchParams({
    project_id: project,
    type,
    status,
    endpoint_id: endpoint,
    offset: String(offset),
    limit: "25",
  });
  if (date) params.set("since", new Date(date).toISOString());
  const list = useLoad<Page<Event | Delivery>>(`/api/${kind}?${params}`);
  useEffect(() => setOffset(0), [project, type, status, endpoint, date, kind]);
  return (
    <>
      <Heading
        title={
          kind === "events"
            ? "Events"
            : kind === "deliveries"
              ? "Deliveries"
              : "Dead letters"
        }
        description={
          kind === "dead-letters"
            ? "Exhausted or permanent failures. Inspect a delivery to replay it."
            : "Filter and inspect the durable record of your webhook activity."
        }
      >
        <button onClick={list.reload}>Refresh</button>
      </Heading>
      <div className="filters">
        <ProjectSelect value={project} onChange={setProject} />
        <Field label="Event type">
          <input
            value={type}
            onChange={(e) => setType(e.target.value)}
            placeholder="order.created"
          />
        </Field>
        {kind === "deliveries" && (
          <Field label="Status">
            <select value={status} onChange={(e) => setStatus(e.target.value)}>
              <option value="">All statuses</option>
              {[
                "pending",
                "processing",
                "retrying",
                "succeeded",
                "dead",
                "cancelled",
              ].map((s) => (
                <option key={s}>{s}</option>
              ))}
            </select>
          </Field>
        )}
        {kind !== "events" && (
          <Field label="Endpoint ID">
            <input
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
            />
          </Field>
        )}
        <Field label="Received after">
          <input
            type="datetime-local"
            value={date}
            onChange={(e) => setDate(e.target.value)}
          />
        </Field>
      </div>
      <section className="panel">
        <State {...list} empty={!list.data?.items.length}>
          {kind === "events" ? (
            <EventTable items={(list.data?.items ?? []) as Event[]} />
          ) : (
            <DeliveryTable items={(list.data?.items ?? []) as Delivery[]} />
          )}
        </State>
        <Pager
          offset={offset}
          count={list.data?.items.length ?? 0}
          onChange={setOffset}
        />
      </section>
    </>
  );
}
function EventDetails() {
  const { id = "" } = useParams();
  const data = useLoad<Event>(`/api/events/${id}`);
  return (
    <>
      <Heading title="Event details" description={data.data?.event_id ?? id} />
      <State {...data}>
        <section className="panel">
          <h2>{data.data?.type}</h2>
          <pre>{JSON.stringify(data.data?.payload, null, 2)}</pre>
        </section>
        <section className="panel">
          <h2>Destination deliveries</h2>
          {data.data?.deliveries?.map((d) => (
            <p className="delivery-link" key={d.id}>
              <Link to={`/deliveries/${d.id}`}>{d.id}</Link>
              <Badge status={d.status} />
            </p>
          ))}
        </section>
      </State>
    </>
  );
}
function DeliveryDetails() {
  const { id = "" } = useParams();
  const data = useLoad<Delivery>(`/api/deliveries/${id}`);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function replay() {
    setBusy(true);
    setError("");
    try {
      await post(`/api/deliveries/${id}/replay`, {});
      data.reload();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <Heading title="Delivery details" description={id}>
        <button onClick={data.reload}>Refresh</button>
      </Heading>
      <ErrorBox error={error} />
      <State {...data}>
        <section className="panel delivery-summary">
          <Badge status={data.data?.status ?? ""} />
          <h2>{data.data?.endpoint_name}</h2>
          <p>
            Replay generation {data.data?.generation} ·{" "}
            {data.data?.attempt_count} attempts in this generation
          </p>
          <Link to={`/events/${data.data?.event_id}`}>View source event →</Link>
          {data.data?.status === "dead" && (
            <button className="primary" disabled={busy} onClick={replay}>
              Replay delivery
            </button>
          )}
        </section>
        <section className="panel">
          <h2>Attempt timeline</h2>
          {!data.data?.attempts?.length ? (
            <p>No completed attempts yet. Refresh after the worker runs.</p>
          ) : (
            <table>
              <thead>
                <tr>
                  <th>Generation / attempt</th>
                  <th>Outcome</th>
                  <th>Latency</th>
                  <th>Time</th>
                </tr>
              </thead>
              <tbody>
                {data.data.attempts.map((a) => (
                  <tr key={a.id}>
                    <td>
                      {a.generation} / {a.attempt}
                    </td>
                    <td>
                      <Badge
                        status={
                          a.status_code >= 200 && a.status_code < 300
                            ? "succeeded"
                            : "dead"
                        }
                      />
                      <small>{a.error || `HTTP ${a.status_code}`}</small>
                    </td>
                    <td>{a.duration_ms} ms</td>
                    <td>{timestamp(a.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      </State>
    </>
  );
}
