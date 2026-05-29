const sampleIDF = `Version,
  24.1;                    !- Version Identifier

ScheduleTypeLimits,
  Fraction;                 !- Name

Schedule:Compact,
  AlwaysOn,                 !- Name
  Fraction,                 !- Schedule Type Limits Name
  Through: 12/31,           !- Field 1
  For: AllDays,             !- Field 2
  Until: 24:00,             !- Field 3
  1;                        !- Field 4

Zone,
  Office;                   !- Name

BuildingSurface:Detailed,
  Office Floor,             !- Name
  Floor,                    !- Surface Type
  FLOOR,                    !- Construction Name
  Office,                   !- Zone Name
  Ground,                   !- Outside Boundary Condition
  ,                         !- Outside Boundary Condition Object
  NoSun,                    !- Sun Exposure
  NoWind,                   !- Wind Exposure
  0.5,                      !- View Factor to Ground
  4,                        !- Number of Vertices
  0,0,0,                    !- X,Y,Z Vertex 1
  10,0,0,                   !- X,Y,Z Vertex 2
  10,8,0,                   !- X,Y,Z Vertex 3
  0,8,0;                    !- X,Y,Z Vertex 4

Lights,
  Office Lights,            !- Name
  Office,                   !- Zone or ZoneList Name
  AlwaysOn,                 !- Schedule Name
  LightingLevel,            !- Design Level Calculation Method
  500;                      !- Lighting Level

Fan:ConstantVolume,
  Supply Fan,               !- Name
  AlwaysOn,                 !- Availability Schedule Name
  0.7,                      !- Fan Total Efficiency
  500,                      !- Pressure Rise
  1.0,                      !- Maximum Flow Rate
  0.9,                      !- Motor Efficiency
  1.0,                      !- Motor In Airstream Fraction
  Air Inlet Node,           !- Air Inlet Node Name
  Air Outlet Node;          !- Air Outlet Node Name
`;

const state = {
  report: null,
  model: null,
  epjsonText: "",
  activeTab: "summary",
  activeInputView: "text",
  lastAnalyzedText: "",
};

const elements = {
  runtimeStatus: document.querySelector("#runtimeStatus"),
  fileInput: document.querySelector("#fileInput"),
  analyzeButton: document.querySelector("#analyzeButton"),
  removeUnusedButton: document.querySelector("#removeUnusedButton"),
  toIDFButton: document.querySelector("#toIDFButton"),
  toEPJSONButton: document.querySelector("#toEPJSONButton"),
  downloadButton: document.querySelector("#downloadButton"),
  guideButton: document.querySelector("#guideButton"),
  idfInput: document.querySelector("#idfInput"),
  textStats: document.querySelector("#textStats"),
  fieldStats: document.querySelector("#fieldStats"),
  jsonStructuredView: document.querySelector("#jsonStructuredView"),
  fieldFilter: document.querySelector("#fieldFilter"),
  fieldTable: document.querySelector("#fieldTable"),
  inputViewButtons: document.querySelectorAll(".view-tab"),
  inputViews: document.querySelectorAll(".input-view"),
  objectCount: document.querySelector("#objectCount"),
  typeCount: document.querySelector("#typeCount"),
  scheduleCount: document.querySelector("#scheduleCount"),
  unusedCount: document.querySelector("#unusedCount"),
  typeList: document.querySelector("#typeList"),
  zoneViz: document.querySelector("#zoneViz"),
  systemViz: document.querySelector("#systemViz"),
  objectTable: document.querySelector("#objectTable"),
  objectFilter: document.querySelector("#objectFilter"),
  scheduleList: document.querySelector("#scheduleList"),
  unusedList: document.querySelector("#unusedList"),
  connectionList: document.querySelector("#connectionList"),
  tabs: document.querySelectorAll(".tab"),
  panes: document.querySelectorAll(".tab-pane"),
};

function backend() {
  return window.go && window.go.main && window.go.main.App;
}

function setStatus(message, tone = "muted") {
  elements.runtimeStatus.textContent = message;
  const colors = {
    muted: "#60707c",
    ok: "#246b44",
    warn: "#a85f00",
    error: "#b3261e",
  };
  elements.runtimeStatus.style.color = colors[tone] || colors.muted;
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

async function analyze() {
  const api = backend();
  updateTextStats();
  if (!api) {
    setStatus("Run with Go/Wails to enable IDF or epJSON analysis", "warn");
    renderEmpty();
    return;
  }

  try {
    const text = elements.idfInput.value;
    const result =
      typeof api.AnalyzeInputText === "function"
        ? await api.AnalyzeInputText(text)
        : { report: await api.AnalyzeIDFText(text), model: null, epjson: "" };
    state.report = result.report;
    state.model = result.model || null;
    state.epjsonText = result.epjson || "";
    state.lastAnalyzedText = text;
    renderReport();
    setStatus("Analysis complete", "ok");
  } catch (error) {
    setStatus(error.message || String(error), "error");
  }
}

async function removeUnused() {
  const api = backend();
  if (!api) {
    setStatus("Backend unavailable", "warn");
    return;
  }

  try {
    const result = await api.RemoveUnusedObjectsText(elements.idfInput.value);
    elements.idfInput.value = result.text;
    updateTextStats();
    await analyze();
    setStatus("Unused objects removed", "ok");
  } catch (error) {
    setStatus(error.message || String(error), "error");
  }
}

async function convertInput(targetFormat) {
  const api = backend();
  if (!api || typeof api.ConvertInputText !== "function") {
    setStatus("Backend unavailable", "warn");
    return;
  }

  try {
    const result = await api.ConvertInputText(elements.idfInput.value, targetFormat);
    elements.idfInput.value = result.text;
    updateTextStats();
    await analyze();
    const label = targetFormat === "epjson" ? "epJSON" : "IDF";
    setStatus(`Converted to ${label}`, "ok");
  } catch (error) {
    setStatus(error.message || String(error), "error");
  }
}

function updateTextStats() {
  const text = elements.idfInput.value;
  const lines = text.length === 0 ? 0 : text.split(/\r\n|\r|\n/).length;
  elements.textStats.textContent = `${lines} lines`;
}

function renderReport() {
  const report = state.report;
  if (!report) {
    renderEmpty();
    return;
  }

  elements.objectCount.textContent = report.objectCount ?? 0;
  elements.typeCount.textContent = report.typeCounts?.length ?? 0;
  elements.scheduleCount.textContent = report.schedules?.length ?? 0;
  elements.unusedCount.textContent = report.unusedObjects?.length ?? 0;

  renderTypeList(report.typeCounts || []);
  renderZoneViz(report.zones || []);
  renderObjectTable(report.objects || []);
  renderScheduleList(report.schedules || []);
  renderUnusedList(report.unusedObjects || []);
  renderSystemViz(report.hvacConnections || []);
  renderConnectionList(report.hvacConnections || []);
  renderInputViews();
}

function renderEmpty() {
  elements.objectCount.textContent = "0";
  elements.typeCount.textContent = "0";
  elements.scheduleCount.textContent = "0";
  elements.unusedCount.textContent = "0";
  elements.typeList.innerHTML = `<div class="empty">No analysis yet</div>`;
  elements.objectTable.innerHTML = `<div class="empty">No objects yet</div>`;
  elements.scheduleList.innerHTML = `<div class="empty">No schedules yet</div>`;
  elements.unusedList.innerHTML = `<div class="empty">No unused objects yet</div>`;
  elements.connectionList.innerHTML = `<div class="empty">No connections yet</div>`;
  elements.zoneViz.innerHTML = "";
  elements.systemViz.innerHTML = "";
  elements.jsonStructuredView.innerHTML = `<div class="empty">No structured input yet</div>`;
  elements.fieldTable.innerHTML = `<div class="empty">No field table yet</div>`;
  elements.fieldStats.textContent = "0 fields";
}

function renderTypeList(typeCounts) {
  elements.typeList.innerHTML = typeCounts.length
    ? typeCounts
        .map(
          (item) => `
            <div class="list-row">
              <span class="row-main" title="${escapeHTML(item.type)}">${escapeHTML(item.type)}</span>
              <span class="badge">${escapeHTML(item.count)}</span>
            </div>`,
        )
        .join("")
    : `<div class="empty">No object types</div>`;
}

function renderObjectTable(objects) {
  const filter = elements.objectFilter.value.trim().toLowerCase();
  const filtered = objects.filter((object) => {
    const haystack = `${object.index} ${object.type} ${object.name || ""}`.toLowerCase();
    return haystack.includes(filter);
  });

  const rows = filtered
    .map(
      (object) => `
        <div class="table-row">
          <span class="row-sub">#${escapeHTML(object.index)}</span>
          <span class="row-main" title="${escapeHTML(object.type)}">${escapeHTML(object.type)}</span>
          <span class="row-main" title="${escapeHTML(object.name || "")}">${escapeHTML(object.name || "-")}</span>
          <span class="badge">${escapeHTML(object.fieldCount)}</span>
        </div>`,
    )
    .join("");

  elements.objectTable.innerHTML = `
    <div class="table-row table-head">
      <span>Index</span>
      <span>Type</span>
      <span>Name</span>
      <span>Fields</span>
    </div>
    ${rows || `<div class="empty">No matching objects</div>`}
  `;
}

function renderScheduleList(schedules) {
  elements.scheduleList.innerHTML = schedules.length
    ? schedules
        .map(
          (schedule) => `
            <div class="list-row">
              <span>
                <span class="row-main" title="${escapeHTML(schedule.name)}">${escapeHTML(schedule.name)}</span>
                <span class="row-sub">${escapeHTML(schedule.type)}</span>
              </span>
              <span class="badge">#${escapeHTML(schedule.index)}</span>
            </div>`,
        )
        .join("")
    : `<div class="empty">No schedules</div>`;
}

function renderUnusedList(unusedObjects) {
  elements.unusedList.innerHTML = unusedObjects.length
    ? unusedObjects
        .map(
          (object) => `
            <div class="list-row">
              <span>
                <span class="row-main" title="${escapeHTML(object.name)}">${escapeHTML(object.name)}</span>
                <span class="row-sub">${escapeHTML(object.type)}</span>
              </span>
              <span class="badge">#${escapeHTML(object.index)}</span>
            </div>`,
        )
        .join("")
    : `<div class="empty">No unused named objects</div>`;
}

function renderConnectionList(connections) {
  elements.connectionList.innerHTML = connections.length
    ? connections
        .map(
          (connection) => `
            <div class="list-row">
              <span>
                <span class="row-main">${escapeHTML(connection.fromNode)} -> ${escapeHTML(connection.toNode)}</span>
                <span class="row-sub">${escapeHTML(connection.objectType)} ${escapeHTML(connection.objectName || "")}</span>
              </span>
              <span class="badge">#${escapeHTML(connection.objectIndex)}</span>
            </div>`,
        )
        .join("")
    : `<div class="empty">No node-to-node connections</div>`;
}

function renderInputViews() {
  if (state.activeInputView === "json") {
    renderJSONView();
  }
  if (state.activeInputView === "table") {
    renderFieldTable();
  }
}

function renderJSONView() {
  const model = state.model;
  if (!model || !Array.isArray(model.objects)) {
    elements.jsonStructuredView.innerHTML = `<div class="empty">Analyze input to build JSON view</div>`;
    return;
  }

  const groups = [];
  const byType = new Map();
  model.objects.forEach((object) => {
    if (!byType.has(object.type)) {
      const group = { type: object.type, objects: [] };
      groups.push(group);
      byType.set(object.type, group);
    }
    byType.get(object.type).objects.push(object);
  });

  const versionLabel = model.version?.raw || "unknown";
  const epjsonPreview = state.epjsonText
    ? `<details class="json-code">
        <summary>epJSON</summary>
        <pre>${escapeHTML(state.epjsonText)}</pre>
      </details>`
    : "";

  elements.jsonStructuredView.innerHTML = `
    <div class="json-meta">
      <span class="badge">${escapeHTML(model.format || "unknown")}</span>
      <span class="badge">Version ${escapeHTML(versionLabel)}</span>
      <span class="badge">${escapeHTML(model.objects.length)} objects</span>
    </div>
    <div class="json-groups">
      ${groups
        .map(
          (group, index) => `
            <details class="json-group" ${index < 4 ? "open" : ""}>
              <summary>
                <span>${escapeHTML(group.type)}</span>
                <span class="badge">${escapeHTML(group.objects.length)}</span>
              </summary>
              ${group.objects.map(renderJSONObject).join("")}
            </details>`,
        )
        .join("")}
    </div>
    ${epjsonPreview}
  `;
}

function renderJSONObject(object) {
  const fields = object.fields || [];
  return `
    <section class="json-object">
      <div class="json-object-head">
        <strong title="${escapeHTML(object.name || "")}">${escapeHTML(object.name || "(unnamed)")}</strong>
        <span class="row-sub">#${escapeHTML(object.sourceIndex ?? "")}</span>
      </div>
      <dl>
        ${fields
          .map(
            (field) => `
              <dt title="${escapeHTML(field.key || field.comment || "")}">${escapeHTML(field.key || field.comment || "field")}</dt>
              <dd title="${escapeHTML(formatJSONValue(field.value))}">${escapeHTML(formatJSONValue(field.value))}</dd>`,
          )
          .join("")}
      </dl>
    </section>
  `;
}

function formatJSONValue(value) {
  if (value === null || value === undefined) {
    return "";
  }
  if (typeof value === "object") {
    return JSON.stringify(value);
  }
  return String(value);
}

function renderFieldTable() {
  const report = state.report;
  if (!report || !Array.isArray(report.objects)) {
    elements.fieldTable.innerHTML = `<div class="empty">Analyze input to build table view</div>`;
    elements.fieldStats.textContent = "0 fields";
    return;
  }

  const filter = elements.fieldFilter.value.trim().toLowerCase();
  const rows = [];
  report.objects.forEach((object) => {
    (object.fields || []).forEach((field, fieldIndex) => {
      const value = field.value || "";
      const comment = field.comment || "";
      const haystack = `${object.index} ${object.type} ${object.name || ""} ${fieldIndex} ${comment} ${value}`.toLowerCase();
      if (filter && !haystack.includes(filter)) {
        return;
      }
      rows.push({ object, field, fieldIndex });
    });
  });

  elements.fieldStats.textContent = `${rows.length} fields`;
  if (!rows.length) {
    elements.fieldTable.innerHTML = `<div class="empty">No matching fields</div>`;
    return;
  }

  elements.fieldTable.innerHTML = `
    <table>
      <thead>
        <tr>
          <th>Obj</th>
          <th>Type</th>
          <th>Name</th>
          <th>Field</th>
          <th>Value</th>
        </tr>
      </thead>
      <tbody>
        ${rows
          .map(({ object, field, fieldIndex }) => {
            const value = field.value || "";
            const label = field.comment || `Field ${fieldIndex + 1}`;
            return `
              <tr>
                <td>#${escapeHTML(object.index)}</td>
                <td title="${escapeHTML(object.type)}">${escapeHTML(object.type)}</td>
                <td title="${escapeHTML(object.name || "")}">${escapeHTML(object.name || "-")}</td>
                <td title="${escapeHTML(label)}">${escapeHTML(label)}</td>
                <td>
                  <input class="field-value-input" data-object-index="${escapeHTML(object.index)}"
                    data-field-index="${escapeHTML(fieldIndex)}" data-original="${escapeHTML(value)}"
                    value="${escapeHTML(value)}" />
                </td>
              </tr>`;
          })
          .join("")}
      </tbody>
    </table>
  `;

  elements.fieldTable.querySelectorAll(".field-value-input").forEach((input) => {
    input.addEventListener("blur", () => applyTableValue(input));
    input.addEventListener("keydown", (event) => {
      if (event.key === "Enter") {
        event.preventDefault();
        input.blur();
      }
      if (event.key === "Escape") {
        input.value = input.dataset.original || "";
        input.blur();
      }
    });
  });
}

async function applyTableValue(input) {
  const nextValue = input.value;
  if (nextValue === input.dataset.original) {
    return;
  }

  const api = backend();
  if (!api || typeof api.UpdateFieldText !== "function") {
    setStatus("Backend unavailable", "warn");
    input.value = input.dataset.original || "";
    return;
  }

  try {
    const result = await api.UpdateFieldText(
      elements.idfInput.value,
      Number(input.dataset.objectIndex),
      Number(input.dataset.fieldIndex),
      nextValue,
    );
    elements.idfInput.value = result.text;
    updateTextStats();
    await analyze();
    setStatus("Field updated", "ok");
  } catch (error) {
    input.value = input.dataset.original || "";
    setStatus(error.message || String(error), "error");
  }
}

function renderZoneViz(zones) {
  const svg = elements.zoneViz;
  const width = 560;
  const height = 260;
  svg.setAttribute("viewBox", `0 0 ${width} ${height}`);

  if (!zones.length) {
    svg.innerHTML = `<text x="24" y="48" fill="#60707c" font-size="14">No zones</text>`;
    return;
  }

  const columns = Math.min(3, zones.length);
  const cellWidth = (width - 48) / columns;
  const cellHeight = 78;
  const content = zones
    .map((zone, index) => {
      const col = index % columns;
      const row = Math.floor(index / columns);
      const x = 24 + col * cellWidth;
      const y = 28 + row * (cellHeight + 18);
      const surfaceText = `${zone.surfaceCount || 0} surfaces`;
      return `
        <g>
          <rect x="${x}" y="${y}" width="${cellWidth - 14}" height="${cellHeight}" rx="6" fill="#e9f5f6" stroke="#007c89" />
          <text x="${x + 12}" y="${y + 30}" fill="#18222b" font-size="14" font-weight="700">${escapeHTML(zone.name)}</text>
          <text x="${x + 12}" y="${y + 54}" fill="#60707c" font-size="12">${escapeHTML(surfaceText)}</text>
        </g>`;
    })
    .join("");
  svg.innerHTML = content;
}

function renderSystemViz(connections) {
  const svg = elements.systemViz;
  const width = 800;
  const height = 260;
  svg.setAttribute("viewBox", `0 0 ${width} ${height}`);

  if (!connections.length) {
    svg.innerHTML = `<text x="24" y="48" fill="#60707c" font-size="14">No HVAC connections</text>`;
    return;
  }

  const nodes = [...new Set(connections.flatMap((item) => [item.fromNode, item.toNode]))].slice(0, 9);
  const spacing = (width - 100) / Math.max(nodes.length - 1, 1);
  const y = 112;
  const nodeX = new Map(nodes.map((node, index) => [node, 50 + index * spacing]));

  const paths = connections
    .filter((connection) => nodeX.has(connection.fromNode) && nodeX.has(connection.toNode))
    .map((connection) => {
      const x1 = nodeX.get(connection.fromNode);
      const x2 = nodeX.get(connection.toNode);
      const mid = (x1 + x2) / 2;
      return `
        <path d="M ${x1} ${y} C ${mid} ${y - 52}, ${mid} ${y - 52}, ${x2} ${y}"
          fill="none" stroke="#a85f00" stroke-width="2" marker-end="url(#arrow)" />`;
    })
    .join("");

  const nodeMarks = nodes
    .map((node) => {
      const x = nodeX.get(node);
      const label = node.length > 18 ? `${node.slice(0, 16)}...` : node;
      return `
        <g>
          <circle cx="${x}" cy="${y}" r="15" fill="#ffffff" stroke="#007c89" stroke-width="2" />
          <text x="${x}" y="${y + 36}" text-anchor="middle" fill="#18222b" font-size="12">${escapeHTML(label)}</text>
        </g>`;
    })
    .join("");

  svg.innerHTML = `
    <defs>
      <marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
        <path d="M 0 0 L 8 4 L 0 8 z" fill="#a85f00"></path>
      </marker>
    </defs>
    ${paths}
    ${nodeMarks}
  `;
}

function downloadText() {
  const blob = new Blob([elements.idfInput.value], { type: "text/plain" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = state.model?.format === "epjson" ? "model.epJSON" : "model.idf";
  link.click();
  URL.revokeObjectURL(url);
}

function openGuide() {
  window.location.assign("./guide.html");
}

function switchTab(tabName) {
  state.activeTab = tabName;
  elements.tabs.forEach((tab) => {
    tab.classList.toggle("active", tab.dataset.tab === tabName);
  });
  elements.panes.forEach((pane) => {
    pane.classList.toggle("active", pane.id === `${tabName}Pane`);
  });
}

async function switchInputView(viewName) {
  state.activeInputView = viewName;
  elements.inputViewButtons.forEach((button) => {
    button.classList.toggle("active", button.dataset.inputView === viewName);
  });
  elements.inputViews.forEach((view) => {
    view.classList.toggle("active", view.id === `${viewName}InputView`);
  });

  if (viewName !== "text" && state.lastAnalyzedText !== elements.idfInput.value) {
    await analyze();
    return;
  }
  renderInputViews();
}

elements.fileInput.addEventListener("change", async (event) => {
  const [file] = event.target.files || [];
  if (!file) {
    return;
  }
  elements.idfInput.value = await file.text();
  updateTextStats();
  await analyze();
});

elements.analyzeButton.addEventListener("click", analyze);
elements.removeUnusedButton.addEventListener("click", removeUnused);
elements.toIDFButton.addEventListener("click", () => convertInput("idf"));
elements.toEPJSONButton.addEventListener("click", () => convertInput("epjson"));
elements.downloadButton.addEventListener("click", downloadText);
elements.guideButton.addEventListener("click", openGuide);
elements.idfInput.addEventListener("input", () => {
  updateTextStats();
  state.lastAnalyzedText = "";
});
elements.objectFilter.addEventListener("input", () => {
  if (state.report) {
    renderObjectTable(state.report.objects || []);
  }
});
elements.fieldFilter.addEventListener("input", renderFieldTable);
elements.tabs.forEach((tab) => {
  tab.addEventListener("click", () => switchTab(tab.dataset.tab));
});
elements.inputViewButtons.forEach((button) => {
  button.addEventListener("click", () => switchInputView(button.dataset.inputView));
});

elements.idfInput.value = sampleIDF;
updateTextStats();
renderEmpty();
analyze();
