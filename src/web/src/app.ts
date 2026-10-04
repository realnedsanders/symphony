type Goal = {
  parts?: { name: string }[];
  interfaces?: { name: string; shape: string }[];
  owners?: { part: string; owner: string }[];
  sensors?: { name: string; observes: string }[];
  decisions?: { name: string; rationale: string }[];
};

type Gap = { id: string; subject: string; summary: string; priority: number };
type Standup = { blockers?: string[]; currentMovement?: string[] };
type Retro = { shipped?: string[]; rejected?: { alternative: string; rationale: string }[] };

type State = {
  goal: Goal;
  gaps: Gap[];
  standups: Standup[];
  retros: Retro[];
};

async function postJSON<T>(url: string, body: unknown): Promise<T> {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<T>;
}

function formValues(form: HTMLFormElement): Record<string, string> {
  const data = new FormData(form);
  const values: Record<string, string> = {};
  for (const [key, value] of data.entries()) {
    values[key] = String(value);
  }
  return values;
}

function show(id: string, text: string): void {
  const node = document.querySelector(id);
  if (node) {
    node.textContent = text;
  }
}

async function loadState(): Promise<void> {
  const response = await fetch("/api/state");
  if (!response.ok) {
    throw new Error(await response.text());
  }
  const state = (await response.json()) as State;
  const parts = (state.goal.parts ?? []).map((part) => part.name).join(", ");
  show("#goal-state", parts === "" ? "No goal yet." : parts);
  const gaps = (state.gaps ?? []).map((gap) => `${gap.id} ${gap.summary} priority ${gap.priority}`).join("\n");
  show("#groom-state", gaps === "" ? "No gaps yet." : gaps);
  const blockers = (state.standups ?? []).flatMap((standup) => standup.blockers ?? []);
  show("#standup-state", blockers.length === 0 ? "No standup yet." : blockers.join("\n"));
  const retros = (state.retros ?? []).map((retro) => {
    const shipped = (retro.shipped ?? []).join(", ");
    const rejected = (retro.rejected ?? []).map((item) => `${item.alternative}: ${item.rationale}`).join("; ");
    return `shipped ${shipped}; rejected ${rejected}`;
  });
  show("#retro-state", retros.length === 0 ? "No retro yet." : retros.join("\n"));
}

function requireForm(id: string): HTMLFormElement {
  const form = document.querySelector(id);
  if (!(form instanceof HTMLFormElement)) {
    throw new Error(`missing form ${id}`);
  }
  return form;
}

requireForm("#goal-form").addEventListener("submit", (event) => {
  event.preventDefault();
  const values = formValues(requireForm("#goal-form"));
  void postJSON<Goal>("/api/goal", values).then((goal) => {
    const names = (goal.parts ?? []).map((part) => part.name).join(", ");
    show("#goal-state", names);
  });
});

requireForm("#groom-form").addEventListener("submit", (event) => {
  event.preventDefault();
  const values = formValues(requireForm("#groom-form"));
  void postJSON<Gap[]>("/api/gaps", {
    id: values.id,
    subject: values.subject,
    summary: values.summary,
    priority: Number(values.priority),
  }).then(() => loadState());
});

requireForm("#standup-form").addEventListener("submit", (event) => {
  event.preventDefault();
  const values = formValues(requireForm("#standup-form"));
  void postJSON<Standup>("/api/standup", { id: "standup", blocker: values.blocker }).then((standup) => {
    show("#standup-state", (standup.blockers ?? []).join("\n"));
  });
});

requireForm("#retro-form").addEventListener("submit", (event) => {
  event.preventDefault();
  void postJSON<Retro>("/api/retro", {}).then((retro) => {
    const shipped = (retro.shipped ?? []).join(", ");
    const rejected = (retro.rejected ?? []).map((item) => `${item.alternative}: ${item.rationale}`).join("; ");
    show("#retro-state", `shipped ${shipped}; rejected ${rejected}`);
  });
});

void loadState();
