(function(){
function memoryItems(context){ return Array.isArray(context.state.groceryMemory)?context.state.groceryMemory:[]; }
function visibleItems(context){ return memoryItems(context).filter(item=>!item.hidden); }
function memoryUseText(item){
  const uses=Number(item.uses||0);
  const parts=[];
  if(item.pinned)parts.push("Pinned");
  parts.push(uses===1?"Used once":uses>1?`Used ${uses} times`:"Ready to use");
  return parts.join(" · ");
}
function memoryButton(label,className,handler){
  const button=document.createElement("button");
  button.type="button";
  button.className=className;
  button.textContent=label;
  return [button,handler];
}
function bindMemoryButton(context,label,className,handler){
  const [button,fn]=memoryButton(label,className,handler);
  context.bindTap(button,fn);
  return button;
}
async function addQuickItem(context,title){
  try{
    const cache=await context.apiJSON(`/api/todo/lists/${encodeURIComponent(context.activeListID())}/tasks`,"POST",{title});
    context.state.tasks=cache.tasks||[];
    context.renderTasks();
  }catch(error){context.setStatus(`Could not add ${context.listItemLowerLabel(context.activeListID())} · ${error.message}`);}
}
function openManager(context){
  context.state.groceryManage=true;
  context.renderTasks();
}
function closeManager(context){
  context.state.groceryManage=false;
  context.setTitle(context.listTitle(context.activeListID()));
  context.renderTasks();
}
async function mutateMemory(context,body,success){
  try{
    const next=await context.apiJSON("/api/todo/grocery-memory","POST",body);
    context.state.groceryMemory=next.groceryMemory||[];
    context.setStatus(success||"Quick add updated locally.");
    context.renderTasks();
  }catch(error){context.setStatus(error&&error.message?error.message:"Could not update Quick add.");}
}
async function addMemoryItem(context){
  const title=(await context.promptText("Add Quick add item","")).trim();
  if(!title)return;
  await mutateMemory(context,{action:"add",title},`Added “${title}” to Quick add.`);
}
async function editMemoryItem(context,item){
  const title=(await context.promptText("Edit Quick add item",item.title||"")).trim();
  if(!title||title===item.title)return;
  await mutateMemory(context,{action:"edit",key:item.key,title},`Quick add item renamed to “${title}”.`);
}
async function togglePin(context,item){
  const pinned=!item.pinned;
  await mutateMemory(context,{action:"pin",key:item.key,pinned},pinned?`Pinned “${item.title}”.`:`Unpinned “${item.title}”.`);
}
async function hideMemoryItem(context,item){
  const ok=await context.confirmListsAction(
    `Remove “${item.title}” from Quick add?`,
    "Grocery items already on your list are unchanged. You can restore this suggestion later.",
    "Keep suggestion",
    "Remove from Quick add",
  );
  if(!ok)return;
  await mutateMemory(context,{action:"hide",key:item.key},`Removed “${item.title}” from Quick add.`);
}
async function restoreMemoryItem(context,item){
  await mutateMemory(context,{action:"restore",key:item.key},`Restored “${item.title}” to Quick add.`);
}
async function deleteHiddenMemoryItem(context,item){
  const ok=await context.confirmListsAction(
    `Delete “${item.title}” from hidden suggestions?`,
    "This permanently removes the Quick add suggestion and its remembered aliases. Grocery items already on your list are unchanged. A future completed Grocery item with this name may be learned again.",
    "Keep suggestion",
    "Delete suggestion",
  );
  if(!ok)return;
  await mutateMemory(context,{action:"delete",key:item.key},`Deleted “${item.title}” from hidden suggestions.`);
}
function renderQuickAdd(body,context){
  const section=document.createElement("section");
  section.className="grocery-quick-add";
  const head=document.createElement("div");
  head.className="grocery-quick-head";
  head.appendChild(Object.assign(document.createElement("div"),{className:"grocery-quick-title",textContent:"Quick add"}));
  head.appendChild(bindMemoryButton(context,"Manage","grocery-quick-manage",()=>openManager(context)));
  section.appendChild(head);
  const items=visibleItems(context).slice(0,8);
  if(!items.length){
    const empty=document.createElement("div");
    empty.className="grocery-quick-empty";
    empty.textContent="Add reusable grocery items from Manage.";
    section.appendChild(empty);
  }else{
    const grid=document.createElement("div");
    grid.className="grocery-quick-grid";
    for(const item of items){
      const button=document.createElement("button");
      button.type="button";
      button.textContent=item.title;
      button.setAttribute("aria-label",`Add ${item.title} to Grocery`);
      context.bindTap(button,()=>addQuickItem(context,item.title));
      grid.appendChild(button);
    }
    section.appendChild(grid);
  }
  body.appendChild(section);
}
function memoryRow(context,item){
  const row=document.createElement("article");
  row.className="grocery-memory-row";
  const detail=document.createElement("div");
  detail.className="grocery-memory-detail";
  detail.append(
    Object.assign(document.createElement("div"),{className:"grocery-memory-title",textContent:item.title||"Untitled item"}),
    Object.assign(document.createElement("div"),{className:"grocery-memory-meta",textContent:memoryUseText(item)}),
  );
  const actions=document.createElement("div");
  actions.className="grocery-memory-actions";
  actions.append(
    bindMemoryButton(context,"Edit","grocery-memory-action",()=>editMemoryItem(context,item)),
    bindMemoryButton(context,item.pinned?"Unpin":"Pin","grocery-memory-action",()=>togglePin(context,item)),
    bindMemoryButton(context,"Hide","grocery-memory-action grocery-memory-hide",()=>hideMemoryItem(context,item)),
  );
  row.append(detail,actions);
  return row;
}
function hiddenRow(context,item){
  const row=document.createElement("article");
  row.className="grocery-memory-row grocery-memory-hidden";
  const detail=document.createElement("div");
  detail.className="grocery-memory-detail";
  detail.append(
    Object.assign(document.createElement("div"),{className:"grocery-memory-title",textContent:item.title||"Untitled item"}),
    Object.assign(document.createElement("div"),{className:"grocery-memory-meta",textContent:"Hidden from Quick add"}),
  );
  const actions=document.createElement("div");
  actions.className="grocery-memory-actions";
  actions.append(
    bindMemoryButton(context,"Restore","grocery-memory-action",()=>restoreMemoryItem(context,item)),
    bindMemoryButton(context,"Delete","grocery-memory-action grocery-memory-delete",()=>deleteHiddenMemoryItem(context,item)),
  );
  row.append(detail,actions);
  return row;
}
function renderManager(body,context){
  context.setTitle("Quick add items");
  const toolbar=document.createElement("div");
  toolbar.className="grocery-memory-toolbar";
  toolbar.append(
    bindMemoryButton(context,"Add item","grocery-memory-add",()=>addMemoryItem(context)),
    bindMemoryButton(context,"Done","grocery-memory-done",()=>closeManager(context)),
  );
  body.appendChild(toolbar);
  const visible=visibleItems(context);
  if(!visible.length){
    body.appendChild(Object.assign(document.createElement("div"),{className:"grocery-memory-empty",textContent:"Add reusable items for one-tap grocery entry."}));
  }else{
    const list=document.createElement("div");
    list.className="grocery-memory-list";
    for(const item of visible)list.appendChild(memoryRow(context,item));
    body.appendChild(list);
  }
  const hidden=memoryItems(context).filter(item=>item.hidden);
  if(hidden.length){
    const section=document.createElement("section");
    section.className="grocery-memory-hidden-list";
    section.appendChild(Object.assign(document.createElement("div"),{className:"grocery-memory-hidden-title",textContent:`Hidden suggestions (${hidden.length})`}));
    for(const item of hidden)section.appendChild(hiddenRow(context,item));
    body.appendChild(section);
  }
}
window.DashGoGroceryQuickAdd={renderQuickAdd,renderManager};
})();
