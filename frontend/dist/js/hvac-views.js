import { backend, elements, escapeHTML, state } from "./state.js";

export function initializeHVACControls() {
  elements.hvacLoopSelect?.addEventListener("change", () => {
    state.activeHVACLoopId = elements.hvacLoopSelect.value;
    state.activeHVACNodeName = "";
    renderHVAC();
  });
  elements.hvacFilter?.addEventListener("input", () => renderHVAC());
  elements.hvacViewButtons?.forEach((button) => {
    button.addEventListener("click", () => {
      state.activeHVACView = button.dataset.hvacView || "loop";
      renderHVAC();
    });
  });
  elements.hvacGraph?.addEventListener("click", (event) => {
    const editButton = event.target.closest("[data-hvac-edit-key]");
    if (editButton) {
      openHVACApplyDialog(editButton.dataset.hvacEditKey || "");
      return;
    }
    const nodeButton = event.target.closest("[data-hvac-node]");
    if (!nodeButton) {
      return;
    }
    state.activeHVACNodeName = nodeButton.dataset.hvacNode || "";
    renderHVAC();
  });
  elements.hvacApplyClose?.addEventListener("click", closeHVACApplyDialog);
  elements.hvacPreviewApply?.addEventListener("click", previewHVACApply);
  elements.hvacApplyForm?.addEventListener("submit", applyHVACEdit);
  elements.hvacApplyBody?.addEventListener("input", () => {
    state.hvacApplyPreview = null;
    if (elements.hvacConfirmApply) {
      elements.hvacConfirmApply.disabled = true;
    }
    if (elements.hvacApplyStatus) {
      elements.hvacApplyStatus.textContent = "Run preview before applying.";
    }
    const previewList = elements.hvacApplyBody.querySelector("#hvacApplyPreviewList");
    if (previewList) {
      previewList.innerHTML = `<div class="empty">Run preview before applying.</div>`;
    }
  });
}

export function renderHVAC(hvac = state.report?.hvac) {
  if (!elements.hvacStats) {
    return;
  }
  if (!hvac) {
    renderEmptyHVAC();
    return;
  }

  const loops = hvac.loops || [];
  if (!state.activeHVACLoopId || !loops.some((loop) => loop.id === state.activeHVACLoopId)) {
    state.activeHVACLoopId = loops[0]?.id || "";
  }
  const selectedLoop = loops.find((loop) => loop.id === state.activeHVACLoopId) || null;
  const query = hvacQuery();

  elements.hvacStats.textContent = `${hvac.airLoopCount || 0} air loops, ${hvac.plantLoopCount || 0} plant loops, ${hvac.zoneRelationCount || 0} zones`;
  renderHVACLoopSelect(loops);
  renderHVACSummary(hvac);
  renderHVACWarnings(hvac, query);
  renderHVACInspector(hvac, selectedLoop);
  renderHVACViewButtons();

  if (state.activeHVACView === "relation") {
    renderHVACRelations(hvac, query);
  } else if (state.activeHVACView === "diagnostics") {
    renderHVACDiagnostics(hvac, query);
  } else {
    renderHVACLoopView(selectedLoop, query);
  }
}

function renderEmptyHVAC() {
  elements.hvacStats.textContent = "0 loops, 0 zones";
  elements.hvacLoopSelect.innerHTML = "";
  elements.hvacSummary.innerHTML = `<div class="empty">No HVAC analysis yet</div>`;
  elements.hvacGraph.innerHTML = `<div class="empty">No loop graph yet</div>`;
  elements.hvacInspectorStats.textContent = "Select a node or component";
  elements.hvacInspector.innerHTML = `<div class="empty">No inspector data</div>`;
  elements.hvacWarningStats.textContent = "0 warnings";
  elements.hvacWarnings.innerHTML = `<div class="empty">No HVAC warnings</div>`;
}

function renderHVACLoopSelect(loops) {
  elements.hvacLoopSelect.innerHTML = loops.length
    ? loops
        .map(
          (loop) =>
            `<option value="${escapeHTML(loop.id)}" ${loop.id === state.activeHVACLoopId ? "selected" : ""}>${escapeHTML(loop.type)}: ${escapeHTML(loop.name || `#${Number(loop.objectIndex) + 1}`)}</option>`,
        )
        .join("")
    : `<option value="">No loops</option>`;
}

function renderHVACViewButtons() {
  elements.hvacViewButtons.forEach((button) => {
    button.classList.toggle("active", button.dataset.hvacView === state.activeHVACView);
  });
}

function renderHVACSummary(hvac) {
  elements.hvacSummary.innerHTML = `
    <div class="hvac-stat-grid">
      ${renderHVACStat("Loops", hvac.loopCount || 0)}
      ${renderHVACStat("AirLoopHVAC", hvac.airLoopCount || 0)}
      ${renderHVACStat("PlantLoop", hvac.plantLoopCount || 0)}
      ${renderHVACStat("Zone relations", hvac.zoneRelationCount || 0)}
      ${renderHVACStat("Nodes", hvac.nodeCount || 0)}
      ${renderHVACStat("Warnings", hvac.warningCount || 0)}
    </div>`;
}

function renderHVACStat(label, value) {
  return `
    <div class="hvac-stat">
      <span>${escapeHTML(label)}</span>
      <strong>${escapeHTML(value)}</strong>
    </div>`;
}

function renderHVACLoopView(loop, query) {
  if (!loop) {
    elements.hvacGraph.innerHTML = `<div class="empty">No HVAC loops found</div>`;
    return;
  }
  if (query && !loopMatchesQuery(loop, query)) {
    elements.hvacGraph.innerHTML = `<div class="empty">Selected loop does not match filter</div>`;
    return;
  }
  elements.hvacGraph.innerHTML = `
    <div class="hvac-loop-title">
      <div>
        <h3>${escapeHTML(loop.name || loop.type)}</h3>
        <span>${escapeHTML(loop.type)} ${renderObjectLink(loop.objectIndex, loop.type)}</span>
      </div>
      <div class="hvac-loop-meta">
        <span>${escapeHTML((loop.relatedZones || []).length)} zones</span>
        <span>${escapeHTML((loop.relatedLoops || []).length)} cross-loop links</span>
      </div>
    </div>
    <div class="hvac-loop-columns">
      ${renderHVACLoopSide(loop.supplySide)}
      ${renderHVACLoopSide(loop.demandSide)}
    </div>
    ${renderCrossLoopRelations(loop)}`;
}

function renderHVACLoopSide(side = {}) {
  return `
    <section class="hvac-loop-side">
      <div class="hvac-section-head">
        <h3>${escapeHTML(side.name || "Side")}</h3>
        <span>${escapeHTML((side.branches || []).length)} branches</span>
      </div>
      <div class="hvac-node-line">
        ${renderNodePill(side.inletNode, "Inlet")}
        <span class="hvac-arrow">-&gt;</span>
        ${renderNodePill(side.outletNode, "Outlet")}
      </div>
      <div class="hvac-side-meta">
        ${side.branchListName ? `<span>BranchList ${escapeHTML(side.branchListName)}</span>` : ""}
        ${side.connectorListName ? `<span>ConnectorList ${escapeHTML(side.connectorListName)}</span>` : ""}
      </div>
      <div class="hvac-branch-list">
        ${(side.branches || []).map(renderHVACBranch).join("") || `<div class="empty">No branches on this side</div>`}
      </div>
      ${(side.connectors || []).length ? `<div class="hvac-connector-list">${side.connectors.map(renderHVACConnector).join("")}</div>` : ""}
    </section>`;
}

function renderHVACBranch(branch) {
  return `
    <article class="hvac-branch">
      <div class="hvac-branch-head">
        <strong>${escapeHTML(branch.name || "Branch")}</strong>
        ${renderObjectLink(branch.objectIndex, "Branch")}
      </div>
      <div class="hvac-node-line">
        ${renderNodePill(branch.inletNode, "In")}
        <span class="hvac-arrow">-&gt;</span>
        ${renderNodePill(branch.outletNode, "Out")}
      </div>
      <div class="hvac-component-list">
        ${(branch.components || []).map(renderHVACComponent).join("") || `<div class="empty">No components parsed</div>`}
      </div>
      ${(branch.warnings || []).length ? `<div class="hvac-inline-warning">${branch.warnings.map((warning) => escapeHTML(warning.message)).join("<br />")}</div>` : ""}
    </article>`;
}

function renderHVACComponent(component) {
  const existsClass = component.exists ? "" : " missing";
  return `
    <div class="hvac-component${existsClass}">
      <div class="hvac-component-main">
        <strong>${escapeHTML(component.objectName || component.objectType || "Component")}</strong>
        <span>${escapeHTML(component.objectType || "Unknown type")} ${renderObjectLink(component.objectIndex, component.objectType)}</span>
      </div>
      <div class="hvac-node-line compact">
        ${renderNodePill(component.inletNode, "In")}
        <span class="hvac-arrow">-&gt;</span>
        ${renderNodePill(component.outletNode, "Out")}
      </div>
      ${
        component.waterInletNode || component.waterOutletNode
          ? `<div class="hvac-node-line compact water">${renderNodePill(component.waterInletNode, "Water In")}<span class="hvac-arrow">-&gt;</span>${renderNodePill(component.waterOutletNode, "Water Out")}</div>`
          : ""
      }
      ${(component.relatedLoopNames || []).length ? `<small>Cross-loop: ${escapeHTML(component.relatedLoopNames.join(", "))}</small>` : ""}
      ${renderHVACEditableFields(component.editableFields)}
    </div>`;
}

function renderHVACEditableFields(fields = []) {
  if (!fields.length) {
    return "";
  }
  return `
    <div class="hvac-edit-field-list">
      ${fields
        .slice(0, 4)
        .map(
          (field) => `
            <button class="hvac-edit-button" data-hvac-edit-key="${escapeHTML(hvacEditKey(field))}" type="button">
              <span>${escapeHTML(hvacEditLabel(field))}</span>
              <small>${escapeHTML(field.currentValue || "blank")}</small>
            </button>`,
        )
        .join("")}
    </div>`;
}

function renderHVACConnector(connector) {
  return `
    <article class="hvac-connector">
      <strong>${escapeHTML(connector.type)} ${escapeHTML(connector.name)}</strong>
      ${renderObjectLink(connector.objectIndex, connector.type)}
      <div>${(connector.branchNames || []).map((branch) => `<span>${escapeHTML(branch)}</span>`).join("")}</div>
    </article>`;
}

function renderCrossLoopRelations(loop) {
  const relations = loop.relatedLoops || [];
  if (!relations.length) {
    return "";
  }
  return `
    <section class="hvac-cross-loop">
      <div class="hvac-section-head">
        <h3>Cross-Loop Relations</h3>
        <span>${escapeHTML(relations.length)} links</span>
      </div>
      <div class="hvac-relation-list">
        ${relations
          .map(
            (relation) => `
              <div class="hvac-relation-row">
                <strong>${escapeHTML(relation.componentType)} ${escapeHTML(relation.componentName)}</strong>
                <span>${escapeHTML(relation.loopType)} ${escapeHTML(relation.loopName)}</span>
              </div>`,
          )
          .join("")}
      </div>
    </section>`;
}

function renderHVACRelations(hvac, query) {
  const relations = (hvac.zoneRelations || []).filter((relation) => zoneRelationMatchesQuery(relation, query));
  elements.hvacGraph.innerHTML = relations.length
    ? `
      <div class="hvac-relation-table" role="table" aria-label="System zone relations">
        <div class="hvac-relation-table-row head" role="row">
          <span>Zone</span><span>Terminal</span><span>AirLoop</span><span>PlantLoop</span><span>Equipment</span>
        </div>
        ${relations.map(renderHVACZoneRelation).join("")}
      </div>`
    : `<div class="empty">No matching system-zone relations</div>`;
}

function renderHVACZoneRelation(relation) {
  return `
    <div class="hvac-relation-table-row" role="row">
      <span>
        ${renderObjectLink(relation.zoneObjectIndex, "Zone")}
        <strong>${escapeHTML(relation.zoneName)}</strong>
      </span>
      <span>${(relation.terminalUnits || []).map((item) => escapeHTML(item.objectName || item.objectType)).join(", ") || "N/A"}</span>
      <span>${(relation.airLoopNames || []).map(escapeHTML).join(", ") || "N/A"}</span>
      <span>${(relation.plantLoopNames || []).map(escapeHTML).join(", ") || "N/A"}</span>
      <span>${(relation.zoneEquipment || []).map((item) => escapeHTML(item.objectName || item.objectType)).join(", ") || "N/A"}</span>
    </div>`;
}

function renderHVACDiagnostics(hvac, query) {
  const warnings = (hvac.warnings || []).filter((warning) => warningMatchesQuery(warning, query));
  elements.hvacGraph.innerHTML = warnings.length
    ? `<div class="hvac-diagnostic-list">${warnings.map(renderHVACWarning).join("")}</div>`
    : `<div class="empty">${(hvac.warnings || []).length ? "No matching HVAC warnings" : "No HVAC connection warnings"}</div>`;
}

function renderHVACWarnings(hvac, query) {
  const warnings = (hvac.warnings || []).filter((warning) => warningMatchesQuery(warning, query)).slice(0, 8);
  elements.hvacWarningStats.textContent = query
    ? `${warnings.length} matching`
    : `${(hvac.warnings || []).length} warnings`;
  elements.hvacWarnings.innerHTML = warnings.length
    ? warnings.map(renderHVACWarning).join("")
    : `<div class="empty">${(hvac.warnings || []).length ? "No matching warnings" : "No HVAC warnings"}</div>`;
}

function renderHVACWarning(warning) {
  return `
    <article class="hvac-warning ${escapeHTML(warning.severity || "warning")}">
      <div>
        <strong>${escapeHTML(warning.message || "")}</strong>
        <span>${escapeHTML([warning.code, warning.objectType, warning.objectName].filter(Boolean).join(" / "))}</span>
      </div>
      ${renderObjectLink(warning.objectIndex, warning.objectType)}
    </article>`;
}

function renderHVACInspector(hvac, selectedLoop) {
  if (state.activeHVACNodeName) {
    const usages = (hvac.nodeUsages || []).filter((usage) => usage.nodeName === state.activeHVACNodeName);
    elements.hvacInspectorStats.textContent = `${usages.length} uses`;
    elements.hvacInspector.innerHTML = `
      <div class="hvac-inspector-title">
        <strong>${escapeHTML(state.activeHVACNodeName)}</strong>
        <span>Node</span>
      </div>
      ${usages.length ? usages.map(renderNodeUsage).join("") : `<div class="empty">No node usages found</div>`}`;
    return;
  }
  if (!selectedLoop) {
    elements.hvacInspectorStats.textContent = "No loop selected";
    elements.hvacInspector.innerHTML = `<div class="empty">Select a loop</div>`;
    return;
  }
  elements.hvacInspectorStats.textContent = `${(selectedLoop.relatedZones || []).length} zones`;
  elements.hvacInspector.innerHTML = `
    <div class="hvac-inspector-title">
      <strong>${escapeHTML(selectedLoop.name || selectedLoop.type)}</strong>
      <span>${escapeHTML(selectedLoop.type)}</span>
    </div>
    <div class="hvac-inspector-kv"><span>Supply branches</span><strong>${escapeHTML((selectedLoop.supplySide?.branches || []).length)}</strong></div>
    <div class="hvac-inspector-kv"><span>Demand branches</span><strong>${escapeHTML((selectedLoop.demandSide?.branches || []).length)}</strong></div>
    <div class="hvac-inspector-kv"><span>Related zones</span><strong>${escapeHTML((selectedLoop.relatedZones || []).length)}</strong></div>
    <div class="hvac-tag-list">${(selectedLoop.relatedZones || []).map((zone) => `<span>${escapeHTML(zone)}</span>`).join("") || `<span>N/A</span>`}</div>`;
}

function renderNodeUsage(usage) {
  return `
    <div class="hvac-node-usage">
      <div>
        <strong>${escapeHTML(usage.role || "node")}</strong>
        <span>${escapeHTML(usage.fieldName || `Field ${Number(usage.fieldIndex) + 1}`)}</span>
      </div>
      <div>
        <span>${escapeHTML(usage.objectType)} ${escapeHTML(usage.objectName || "")}</span>
        ${renderObjectLink(usage.objectIndex, usage.objectType)}
      </div>
    </div>`;
}

function renderNodePill(nodeName, label) {
  if (!nodeName) {
    return `<span class="hvac-node empty-node">${escapeHTML(label)} N/A</span>`;
  }
  const active = nodeName === state.activeHVACNodeName ? " active" : "";
  return `<button class="hvac-node${active}" data-hvac-node="${escapeHTML(nodeName)}" type="button"><small>${escapeHTML(label)}</small>${escapeHTML(nodeName)}</button>`;
}

function renderObjectLink(objectIndex, objectType) {
  const index = Number(objectIndex);
  if (!Number.isFinite(index) || index < 0) {
    return "";
  }
  return `<button class="profile-object-link navigable-row" data-jump-object-index="${escapeHTML(index)}" data-jump-object-type="${escapeHTML(objectType || "")}" type="button">#${escapeHTML(index + 1)}</button>`;
}

function hvacQuery() {
  return (elements.hvacFilter?.value || "").trim().toLowerCase();
}

function loopMatchesQuery(loop, query) {
  if (!query) {
    return true;
  }
  const haystack = [
    loop.type,
    loop.name,
    ...(loop.relatedZones || []),
    ...loopComponents(loop).flatMap((component) => [
      component.objectType,
      component.objectName,
      component.inletNode,
      component.outletNode,
      component.waterInletNode,
      component.waterOutletNode,
    ]),
  ]
    .join(" ")
    .toLowerCase();
  return haystack.includes(query);
}

function zoneRelationMatchesQuery(relation, query) {
  if (!query) {
    return true;
  }
  return [
    relation.zoneName,
    ...(relation.airLoopNames || []),
    ...(relation.plantLoopNames || []),
    ...(relation.terminalUnits || []).flatMap((item) => [item.objectType, item.objectName]),
    ...(relation.zoneEquipment || []).flatMap((item) => [item.objectType, item.objectName]),
  ]
    .join(" ")
    .toLowerCase()
    .includes(query);
}

function warningMatchesQuery(warning, query) {
  if (!query) {
    return true;
  }
  return [warning.severity, warning.category, warning.code, warning.message, warning.objectType, warning.objectName, warning.field, warning.value]
    .join(" ")
    .toLowerCase()
    .includes(query);
}

function loopComponents(loop) {
  const sides = [loop.supplySide, loop.demandSide].filter(Boolean);
  return sides.flatMap((side) => (side.branches || []).flatMap((branch) => branch.components || []));
}

function hvacEditKey(field) {
  return `${field.objectIndex}:${field.fieldIndex}`;
}

function hvacEditLabel(field) {
  if (field.editKind === "availability_schedule") {
    return "Availability";
  }
  if (field.editKind === "flow") {
    return "Flow";
  }
  if (field.editKind === "capacity") {
    return "Capacity";
  }
  if (field.editKind === "sequence") {
    return "Sequence";
  }
  return field.fieldName || "Field";
}

function allHVACEditableFields(hvac = state.report?.hvac) {
  const loops = hvac?.loops || [];
  const loopFields = loops.flatMap((loop) => loopComponents(loop).flatMap((component) => component.editableFields || []));
  const relationFields = (hvac?.zoneRelations || []).flatMap((relation) =>
    [...(relation.terminalUnits || []), ...(relation.zoneEquipment || [])].flatMap((component) => component.editableFields || []),
  );
  const byKey = new Map();
  [...loopFields, ...relationFields].forEach((field) => byKey.set(hvacEditKey(field), field));
  return [...byKey.values()];
}

function findHVACEditableField(key) {
  return allHVACEditableFields().find((field) => hvacEditKey(field) === key) || null;
}

function openHVACApplyDialog(key) {
  const field = findHVACEditableField(key);
  if (!field) {
    return;
  }
  state.hvacApplyField = field;
  state.hvacApplyPreview = null;
  const listID = "hvacApplyValueSuggestions";
  elements.hvacApplyBody.innerHTML = `
    <section>
      <h4>${escapeHTML(field.objectType)} ${escapeHTML(field.objectName || "")}</h4>
      <p>${escapeHTML(field.impact || "Changes this HVAC field.")}</p>
      <div class="settings-profile-grid">
        <label class="settings-profile-field">
          <span>Field</span>
          <input type="text" value="${escapeHTML(field.fieldName || `Field ${Number(field.fieldIndex) + 1}`)}" readonly />
        </label>
        <label class="settings-profile-field">
          <span>Current</span>
          <input type="text" value="${escapeHTML(field.currentValue || "")}" readonly />
        </label>
        <label class="settings-profile-field">
          <span>New value</span>
          <input id="hvacApplyValue" type="text" value="${escapeHTML(field.currentValue || "")}" list="${listID}" />
          <datalist id="${listID}">
            ${(field.suggestedValues || []).map((item) => `<option value="${escapeHTML(item.value || "")}" label="${escapeHTML(item.label || item.source || "")}"></option>`).join("")}
          </datalist>
        </label>
      </div>
    </section>
    <section>
      <h4>Preview</h4>
      <div id="hvacApplyPreviewList" class="profile-apply-preview"><div class="empty">Run preview before applying.</div></div>
    </section>`;
  elements.hvacApplyStatus.textContent = "Review changes before applying.";
  elements.hvacConfirmApply.disabled = true;
  elements.hvacApplyDialog.classList.remove("hidden");
  elements.hvacApplyBody.querySelector("#hvacApplyValue")?.focus();
}

function closeHVACApplyDialog() {
  elements.hvacApplyDialog.classList.add("hidden");
}

async function previewHVACApply() {
  const request = hvacApplyRequest();
  if (!request) {
    return;
  }
  try {
    elements.hvacApplyStatus.textContent = "Building preview";
    const preview = await callHVACApplyAPI("PreviewHVACApplyText", "/api/hvac-apply-preview", request);
    state.hvacApplyPreview = preview;
    renderHVACApplyPreview(preview);
    elements.hvacConfirmApply.disabled = !preview.canApply;
    elements.hvacApplyStatus.textContent = preview.canApply ? "Preview ready." : "Preview has blocking warnings.";
  } catch (error) {
    elements.hvacApplyStatus.textContent = error?.message || String(error);
    elements.hvacConfirmApply.disabled = true;
  }
}

async function applyHVACEdit(event) {
  event.preventDefault();
  const request = hvacApplyRequest();
  if (!request) {
    return;
  }
  try {
    elements.hvacApplyStatus.textContent = "Applying HVAC change";
    const result = await callHVACApplyAPI("ApplyHVACText", "/api/hvac-apply", request);
    window.dispatchEvent(new CustomEvent("idfAnalyzer:hvacApplied", { detail: result }));
    closeHVACApplyDialog();
  } catch (error) {
    elements.hvacApplyStatus.textContent = error?.message || String(error);
  }
}

function hvacApplyRequest() {
  const field = state.hvacApplyField;
  if (!field) {
    return null;
  }
  const value = elements.hvacApplyBody.querySelector("#hvacApplyValue")?.value ?? "";
  return {
    changes: [
      {
        objectIndex: Number(field.objectIndex),
        fieldIndex: Number(field.fieldIndex),
        value,
      },
    ],
  };
}

async function callHVACApplyAPI(methodName, endpoint, request) {
  const api = backend();
  if (api && typeof api[methodName] === "function") {
    return api[methodName](elements.idfInput.value, request);
  }
  const response = await fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text: elements.idfInput.value, apply: request }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json();
}

function renderHVACApplyPreview(preview) {
  const list = elements.hvacApplyBody.querySelector("#hvacApplyPreviewList");
  if (!list) {
    return;
  }
  const changes = preview?.changes || [];
  const warnings = preview?.warnings || [];
  list.innerHTML = `
    ${warnings.map(renderHVACApplyWarning).join("")}
    ${
      changes.length
        ? changes.map(renderHVACApplyChange).join("")
        : `<div class="empty">${warnings.length ? "No changes can be applied yet" : "No field changes"}</div>`
    }`;
}

function renderHVACApplyChange(change) {
  return `
    <div class="profile-apply-change">
      <strong>${escapeHTML(change.message || "")}</strong>
      <span>${escapeHTML(change.objectType || "")} ${escapeHTML(change.objectName || "")} / ${escapeHTML(change.fieldName || "")}</span>
    </div>`;
}

function renderHVACApplyWarning(warning) {
  return `
    <div class="profile-warning ${escapeHTML(warning.severity || "warning")}">
      <strong>${escapeHTML(warning.code || "warning")}</strong>
      <span>${escapeHTML(warning.message || "")}</span>
    </div>`;
}
