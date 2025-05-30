/**
 * @fileoverview Paxos consensus algorithm visualization
 * @author 0xdvc
 */
"use strict";

const NodeType = {
  PROPOSER: "proposer",
  ACCEPTOR: "acceptor",
  LEARNER: "learner",
};

const MessageType = {
  PREPARE: 1,
  PROMISE: 2,
  PROPOSE: 3,
  ACCEPT: 4,
};

const MessageTypeNames = {
  1: "PREPARE",
  2: "PROMISE",
  3: "PROPOSE",
  4: "ACCEPT",
};

const NodeColors = {
  proposer: "#f39c12",
  acceptor: "#27ae60",
  learner: "#e91e63",
};

const MessageColors = {
  1: "#f39c12",
  2: "#27ae60",
  3: "#9b59b6",
  4: "#e91e63",
};

/**
 * Visualizes Paxos consensus algorithm
 * @class
 */
class PaxosVisualizer {
  constructor() {
    this.canvas = document.getElementById("paxos-canvas");
    this.ctx = this.canvas.getContext("2d");
    this.logElement = document.getElementById("log");
    this.nodes = [];
    this.messages = [];
    this.messageId = 0;
    this.consensusValue = null;
    this.consensusReached = false;
    this.animationFrame = null;
    this.lastTimestamp = 0;
    this.simSpeed = 1.0;
    this.paused = false;
    this.autoRun = false;
    this.apiBaseUrl = "/api";
    this.pollingInterval = null;
    this.pollingDelay = 50;
    this.stats = {
      prepareCount: 0,
      promiseCount: 0,
      proposalCount: 0,
      acceptCount: 0,
    };
    this.currentPhase = "Idle";
    this.initEventListeners();
    this.updateNodeLayout();
    this.render();
    this.updateUI();
    console.log("paxos visualizer initialized");
  }

  /**
   * Sets up UI event listeners
   */
  initEventListeners() {
    document
      .getElementById("start-btn")
      .addEventListener("click", () => this.startSimulation());
    document
      .getElementById("pause-btn")
      .addEventListener("click", () => this.togglePause());
    document
      .getElementById("step-btn")
      .addEventListener("click", () => this.stepSimulation());
    document
      .getElementById("reset-btn")
      .addEventListener("click", () => this.resetSimulation());
    document
      .getElementById("auto-run-btn")
      .addEventListener("click", () => this.toggleAutoRun());
    document.getElementById("sim-speed").addEventListener("input", (e) => {
      this.simSpeed = parseFloat(e.target.value);
      document.getElementById("speed-display").textContent =
        `${this.simSpeed.toFixed(1)}x`;
    });
    document
      .getElementById("acceptor-count")
      .addEventListener("change", (e) => {
        let count = parseInt(e.target.value);
        if (count % 2 === 0) e.target.value = count + 1;
        this.updateNodeLayout();
        this.render();
      });
    document.getElementById("propose-value").addEventListener("change", () => {
      if (!this.consensusReached) {
        this.updateNodeLayout();
        this.render();
      }
    });
  }

  /**
   * Updates node positions
   */
  updateNodeLayout() {
    const width = this.canvas.width;
    const height = this.canvas.height;
    this.nodes = [];
    const acceptorCount =
      parseInt(document.getElementById("acceptor-count").value) || 3;
    const proposeValue =
      document.getElementById("propose-value").value || "50";
    this.nodes.push({
      id: 100,
      type: NodeType.PROPOSER,
      x: width * 0.15,
      y: height * 0.5,
      radius: 45,
      label: "Proposer",
      state: "idle",
      data: {
        value: proposeValue,
        seq: 0,
        promisedCount: 0,
        highestAcceptedSeq: 0,
        highestAcceptedVal: "",
        hasProposed: false,
      },
    });
    for (let i = 0; i < acceptorCount; i++) {
      const yPos =
        acceptorCount > 1
          ? height * 0.2 + (height * 0.6 * i) / (acceptorCount - 1)
          : height * 0.5;
      this.nodes.push({
        id: i + 1,
        type: NodeType.ACCEPTOR,
        x: width * 0.5,
        y: yPos,
        radius: 40,
        label: `Acceptor ${i + 1}`,
        state: "idle",
        data: { promisedSeq: 0, acceptedSeq: 0, acceptedVal: "" },
      });
    }
    this.nodes.push({
      id: 200,
      type: NodeType.LEARNER,
      x: width * 0.85,
      y: height * 0.5,
      radius: 45,
      label: "Learner",
      state: "idle",
      data: { acceptedMsgs: {} },
    });
    window.paxosNodes = this.nodes;
    document.getElementById("total-nodes").textContent = this.nodes.length;
  }

  /**
   * Starts the simulation
   */
  startSimulation() {
    if (this.consensusReached) this.resetSimulation();
    const proposeValue = document.getElementById("propose-value").value;
    const acceptorCount = parseInt(
      document.getElementById("acceptor-count").value,
    );
    this.addLog("=== PAXOS STARTED ===", "network");
    this.addLog(
      `Starting with value: "${proposeValue}" and ${acceptorCount} acceptors`,
      "network",
    );
    this.currentPhase = "Initializing";
    this.updateUI();
    this.startStatePolling();
    
    console.log("starting animation loop...");
    if (!this.animationFrame) {
      this.lastTimestamp = performance.now();
      this.animate(this.lastTimestamp);
    }
    
    fetch(`${this.apiBaseUrl}/start`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ value: proposeValue, acceptorCount }),
    })
      .then((response) => response.json())
      .then((data) => {
        console.log("Simulation started:", data);
      })
      .catch((error) => {
        console.error("Error starting simulation:", error);
        this.addLog("Error starting simulation", "network");
      });
  }

  /**
   * Resets the simulation
   */
  resetSimulation() {
    if (this.animationFrame) {
      cancelAnimationFrame(this.animationFrame);
      this.animationFrame = null;
    }
    if (this.pollingInterval) {
      clearInterval(this.pollingInterval);
      this.pollingInterval = null;
    }
    
    fetch(`${this.apiBaseUrl}/stop`, { method: "POST" })
      .then((response) => response.json())
      .catch((error) => console.error("Error stopping simulation:", error));
    
    this.messages = [];
    this.messageId = 0;
    this.stats = {
      prepareCount: 0,
      promiseCount: 0,
      proposalCount: 0,
      acceptCount: 0,
    };
    this.consensusValue = null;
    this.consensusReached = false;
    this.currentPhase = "idle";
    
    const overlay = document.getElementById("consensus-overlay");
    if (overlay) {
      overlay.textContent = "";
      overlay.classList.remove("visible");
    }
    
    // Reset consensus status
    const consensusStatusElement = document.getElementById("consensus-status");
    if (consensusStatusElement) {
      consensusStatusElement.textContent = "None";
    }
    
    this.updateNodeLayout();
    this.logElement.innerHTML = "";
    this.addLog("Simulation reset and ready to start", "network");
    this.updateUI();
    this.render();
  }

  /**
   * Toggles pause/resume
   */
  togglePause() {
    this.paused = !this.paused;
    document.getElementById("pause-btn").textContent = this.paused
      ? "Resume"
      : "Pause";
    if (!this.paused && !this.animationFrame) {
      this.lastTimestamp = performance.now();
      this.animate(this.lastTimestamp);
    }
  }

  /**
   * Steps through simulation
   */
  stepSimulation() {
    if (!this.paused) this.togglePause();
    this.updateMessages(16.67 * this.simSpeed);
    this.render();
  }

  /**
   * Toggles auto-run mode
   */
  toggleAutoRun() {
    this.autoRun = !this.autoRun;
    document.getElementById("auto-run-btn").textContent = this.autoRun
      ? "Stop Auto"
      : "Auto Run";
    if (this.autoRun) this.runAutoSimulation();
  }

  /**
   * Runs auto simulations
   */
  async runAutoSimulation() {
    if (!this.autoRun) return;
    if (this.consensusReached) {
      this.resetSimulation();
      await new Promise((resolve) => setTimeout(resolve, 1000));
    }
    const randomValue = `value_${Math.floor(Math.random() * 1000)}`;
    document.getElementById("propose-value").value = randomValue;
    this.updateNodeLayout();
    this.startSimulation();
    const startTime = Date.now();
    const checkConsensus = async () => {
      if (!this.autoRun) return;
      if (this.consensusReached) {
        await new Promise((resolve) => setTimeout(resolve, 3000));
        if (this.autoRun) this.runAutoSimulation();
      } else if (Date.now() - startTime > 30000) {
        this.addLog("Auto-run timeout: restarting simulation", "network");
        this.resetSimulation();
        await new Promise((resolve) => setTimeout(resolve, 1000));
        if (this.autoRun) this.runAutoSimulation();
      } else {
        setTimeout(checkConsensus, 500);
      }
    };
    checkConsensus();
  }

  /**
   * Animation loop
   */
  animate(timestamp) {
    if (!this.lastTimestamp) {
      this.lastTimestamp = timestamp;
    }
    
    const deltaTime = timestamp - this.lastTimestamp;
    this.lastTimestamp = timestamp;
    
    if (!this.paused) {
      this.updateMessages(deltaTime);
    }
    
    this.render();
    this.updateUI();
    
    this.animationFrame = requestAnimationFrame((ts) => this.animate(ts));
    
  }

  /**
   * Updates message positions
   * @param {number} deltaTime - Time since last frame
   */
  updateMessages(deltaTime) {
    if (this.messages.length === 0) return;
    
    const duration = 1000 / this.simSpeed; // 1s animation
    const speed = 1 / duration;
    const progressIncrement = speed * deltaTime;
    const arrived = [];
    const remaining = [];
    
    console.log("Updating messages - count:", this.messages.length, "deltaTime:", deltaTime, "progressIncrement:", progressIncrement);
    
    this.messages.forEach((msg) => {
      msg.progress = Math.min(msg.progress + progressIncrement, 1);
      if (msg.progress >= 1) arrived.push(msg);
      else remaining.push(msg);
    });
    
    arrived.forEach((msg) => this.handleMessageArrival(msg));
    this.messages = remaining;
    
    const activeMessagesElement = document.getElementById("active-messages");
    if (activeMessagesElement) {
      activeMessagesElement.textContent = this.messages.length;
    }
    
    console.log("Messages after update - arrived:", arrived.length, "remaining:", this.messages.length);
  }

  /**
   * Finds a node by ID and type
   * @param {number} id - Node ID
   * @param {string} type - Node type
   * @returns {Object|null} Node object
   */
  findNode(id, type) {
    // First try to find by ID and type
    let node = this.nodes.find((n) => n.id === id && n.type === type);
    if (node) return node;
    
    // Fallback: find by ID only (in case type mapping is different)
    node = this.nodes.find((n) => n.id === id);
    if (node) return node;
    
    return null;
  }

  /**
   * Gets acceptor nodes
   * @returns {Object[]} Acceptor nodes
   */
  getAcceptors() {
    return this.nodes.filter((n) => n.type === NodeType.ACCEPTOR);
  }

  /**
   * Calculates majority count
   * @returns {number} Majority threshold
   */
  getMajority() {
    return Math.floor(this.getAcceptors().length / 2) + 1;
  }

  /**
   * Handles message arrival
   * @param {Object} msg - Message object
   */
  handleMessageArrival(msg) {
    if (this.consensusReached) return;
    const fromNode = this.findNode(msg.from, msg.fromType);
    const toNode = this.findNode(msg.to, msg.toType);
    if (!fromNode || !toNode) return;
    
    console.log("Message arrived:", msg.type, "from:", fromNode.label, "to:", toNode.label);
    
    this.addLog(
      `[Network] ${MessageTypeNames[msg.type]} arrived: ${fromNode.label} → ${toNode.label}`,
      "network",
    );
  }

  /**
   * Updates from server state
   * @param {Object} state - Server state
   */
  updateFromServerState(state) {
    console.log("Updating from server state:", state);
    console.log("Current nodes:", this.nodes);
    
    if (
      state.nodes &&
      state.nodes.length > 0
    ) {
      // If node count changed or this is the first update, sync completely with server
      if (state.nodes.length !== this.nodes.length || this.nodes.length === 0) {
        console.log("Syncing nodes with server layout");
        // Clear current nodes and use server's layout
        this.nodes = [];
        state.nodes.forEach((serverNode) => {
          // Map server node types to frontend node types
          let nodeType = serverNode.type;
          if (typeof serverNode.type === 'string') {
            switch (serverNode.type) {
              case 'proposer': nodeType = NodeType.PROPOSER; break;
              case 'acceptor': nodeType = NodeType.ACCEPTOR; break;
              case 'learner': nodeType = NodeType.LEARNER; break;
              default: nodeType = NodeType.ACCEPTOR; break;
            }
          }
          
                  this.nodes.push({
          id: serverNode.id,
          type: nodeType,
          x: serverNode.x,
          y: serverNode.y,
          radius: nodeType === NodeType.ACCEPTOR ? 40 : 45,
          label: serverNode.label,
          state: serverNode.state || "idle",
          data: {
            value: serverNode.value || "",
            promisedSeq: 0,
            acceptedSeq: 0,
            acceptedVal: serverNode.value || "",
            acceptedMsgs: {},
          },
        });
        });
        window.paxosNodes = this.nodes;
        document.getElementById("total-nodes").textContent = this.nodes.length;
        console.log("Updated nodes:", this.nodes);
      }
    }
    (state.nodes || []).forEach((serverNode) => {
      const node = this.findNode(serverNode.id, serverNode.type);
      if (node) {
        // Update node state and data
        node.state = serverNode.state;
        node.data.value = serverNode.value || node.data.value;
        
        // Update node position to match server
        if (serverNode.x !== undefined && serverNode.y !== undefined) {
          node.x = serverNode.x;
          node.y = serverNode.y;
        }
        
        if (node.type === NodeType.ACCEPTOR) {
          node.data.acceptedVal = serverNode.value || "";
          // Update acceptor state text
          if (serverNode.state === "promised") {
            node.data.promisedSeq = serverNode.seq || 0;
          }
        } else if (node.type === NodeType.LEARNER) {
          // Update learner's accepted messages count based on server state
          if (state.acceptCount !== undefined) {
            node.data.acceptedMsgs = {};
            // Create dummy accepted messages for visualization
            for (let i = 0; i < state.acceptCount; i++) {
              node.data.acceptedMsgs[`acceptor_${i}`] = { seq: i + 1, val: "accepted" };
            }
          }
        }
      }
    });
    
    // Update UI elements that depend on node count
    const totalNodesElement = document.getElementById("total-nodes");
    if (totalNodesElement && this.nodes.length > 0) {
      totalNodesElement.textContent = this.nodes.length;
    }
    if (state.messages && state.messages.length > 0) {
      console.log("Received messages from server:", state.messages);
      const newMessages = state.messages.filter(
        (msg) => !this.messages.some((m) => m.id === msg.id),
      );
      console.log("New messages to add:", newMessages);
      
      newMessages.forEach((msg) => {
        const fromNode = this.findNode(msg.from, msg.FromType);
        const toNode = this.findNode(msg.to, msg.ToType);
        console.log("Message from/to nodes:", msg.from, msg.to, fromNode, toNode);
        
        if (fromNode && toNode) {
          // Map server message types to frontend message types
          let messageType = msg.type;
          if (typeof msg.type === 'string') {
            // Handle string message types from server
            switch (msg.type) {
              case 'prepare': messageType = MessageType.PREPARE; break;
              case 'promise': messageType = MessageType.PROMISE; break;
              case 'propose': messageType = MessageType.PROPOSE; break;
              case 'accept': messageType = MessageType.ACCEPT; break;
              default: messageType = MessageType.PREPARE; break;
            }
          }
          
          this.messages.push({
            id: msg.id,
            from: fromNode,
            to: toNode,
            type: messageType,
            fromType: msg.FromType,
            toType: msg.ToType,
            progress: 0,
            val: msg.val,
            seq: msg.seq,
          });
          console.log("Added message:", this.messages[this.messages.length - 1]);
        }
      });
      
      // Remove old messages that are no longer in server state
      this.messages = this.messages.filter(msg => 
        state.messages.some(serverMsg => serverMsg.id === msg.id)
      );
      console.log("Total messages after update:", this.messages.length);
    }
    console.log("Updating stats:", {
      prepareCount: state.prepareCount,
      promiseCount: state.promiseCount,
      proposeCount: state.proposeCount,
      acceptCount: state.acceptCount
    });
    
    this.stats.prepareCount = state.prepareCount || 0;
    this.stats.promiseCount = state.promiseCount || 0;
    this.stats.proposalCount = state.proposeCount || 0;
    this.stats.acceptCount = state.acceptCount || 0;
    if (state.phase) {
      console.log("Updating phase:", state.phase);
      this.currentPhase = state.phase;
      // Update phase display in UI
      const phaseElement = document.getElementById("current-phase");
      if (phaseElement) {
        phaseElement.textContent = state.phase;
      }
    }
    if (state.consensus === "Reached" && state.consensusVal) {
      console.log("Consensus reached:", state.consensusVal);
      this.consensusValue = state.consensusVal;
      this.consensusReached = true;
      const overlay = document.getElementById("consensus-overlay");
      overlay.textContent = `Consensus: "${state.consensusVal}"`;
      overlay.classList.add("visible");
      
      // Update consensus status in UI
      const consensusStatusElement = document.getElementById("consensus-status");
      if (consensusStatusElement) {
        consensusStatusElement.textContent = `"${state.consensusVal}"`;
      }
      
      this.addLog(
        `CONSENSUS REACHED on value: "${state.consensusVal}"`,
        "consensus",
      );
      if (this.pollingInterval) {
        clearInterval(this.pollingInterval);
        this.pollingInterval = null;
      }
    }
    if (state.eventLog && state.eventLog.length > 0) {
      console.log("Received event log entries:", state.eventLog);
      const newLogEntries = state.eventLog.slice(
        Math.max(0, state.eventLog.length - 10),
      );
      console.log("New log entries to add:", newLogEntries);
      
      newLogEntries.forEach((logMsg) => {
        if (
          !Array.from(this.logElement.children).some(
            (el) => el.textContent === logMsg,
          )
        ) {
          const logClass = logMsg.includes("Proposer")
            ? "proposer"
            : logMsg.includes("Acceptor")
              ? "acceptor"
              : logMsg.includes("Learner")
                ? "learner"
                : logMsg.includes("CONSENSUS")
                  ? "consensus"
                  : "network";
          this.addLog(logMsg, logClass);
        }
      });
    }
    this.updateUI();
    this.render();
  }

  /**
   * Adds a log entry
   * @param {string} message - Log message
   * @param {string} className - CSS class
   */
  addLog(message, className = "") {
    const logEntry = document.createElement("div");
    logEntry.className = `log-entry ${className}`;
    logEntry.textContent = message;
    this.logElement.appendChild(logEntry);
    this.logElement.scrollTop = this.logElement.scrollHeight;
  }

  /**
   * Renders the visualization
   */
  render() {
    if (!this.ctx || !this.canvas) return;
    this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
    this.ctx.fillStyle = "#fafafa";
    this.ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
    this.drawConnections();
    this.drawMessages();
    this.drawNodes();
    
    // Debug rendering
    if (this.messages.length > 0) {
      console.log("Rendered - nodes:", this.nodes.length, "messages:", this.messages.length);
    }
  }

  /**
   * Draws node connections
   */
  drawConnections() {
    const proposer = this.nodes.find((n) => n.type === NodeType.PROPOSER);
    const acceptors = this.getAcceptors();
    const learner = this.nodes.find((n) => n.type === NodeType.LEARNER);
    if (!proposer || !learner) return;
    this.ctx.strokeStyle = "#ddd";
    this.ctx.lineWidth = 1;
    acceptors.forEach((acceptor) => {
      this.ctx.beginPath();
      this.ctx.moveTo(proposer.x, proposer.y);
      this.ctx.lineTo(acceptor.x, acceptor.y);
      this.ctx.stroke();
      this.ctx.beginPath();
      this.ctx.moveTo(acceptor.x, acceptor.y);
      this.ctx.lineTo(learner.x, learner.y);
      this.ctx.stroke();
    });
  }

  /**
   * Draws all nodes
   */
  drawNodes() {
    this.nodes.forEach((node) => this.drawNode(node));
  }

  /**
   * Draws a single node
   * @param {Object} node - Node object
   */
  drawNode(node) {
    this.ctx.beginPath();
    this.ctx.arc(node.x, node.y, node.radius, 0, Math.PI * 2);
    this.ctx.fillStyle = NodeColors[node.type] || "#999";
    this.ctx.fill();
    this.ctx.beginPath();
    this.ctx.arc(node.x, node.y, node.radius - 4, 0, Math.PI * 2);
    this.ctx.fillStyle = "white";
    this.ctx.fill();
    
    // Draw main label with smaller font
    this.ctx.fillStyle = "#333";
    this.ctx.font = "bold 12px Arial";
    this.ctx.textAlign = "center";
    this.ctx.textBaseline = "middle";
    
    // Truncate label if it's too long for the circle
    let displayLabel = node.label;
    if (node.label.length > 8) {
      displayLabel = node.label.substring(0, 6) + "..";
    }
    
    this.ctx.fillText(displayLabel, node.x, node.y - 8);
    
    // Draw state text below with even smaller font
    let stateText = "";
    if (node.type === NodeType.PROPOSER) {
      stateText =
        node.data.value.length > 6
          ? `${node.data.value.substring(0, 4)}..`
          : node.data.value;
    } else if (node.type === NodeType.ACCEPTOR) {
      stateText = node.data.acceptedVal
        ? node.data.acceptedVal.length > 6
          ? `${node.data.acceptedVal.substring(0, 4)}..`
          : node.data.acceptedVal
        : node.state === "promised"
          ? `p:${node.data.promisedSeq}`
          : "";
    } else if (node.type === NodeType.LEARNER) {
      const count = Object.keys(node.data.acceptedMsgs).length;
      if (count > 0) stateText = `${count} acc`;
    }
    
    if (stateText) {
      this.ctx.font = "9px Arial";
      this.ctx.fillText(stateText, node.x, node.y + 10);
    }
  }

  /**
   * Draws all messages
   */
  drawMessages() {
    this.messages.forEach((msg) => this.drawMessage(msg));
  }

  /**
   * Draws a single message
   * @param {Object} msg - Message object
   */
  drawMessage(msg) {
    const x = msg.from.x + (msg.to.x - msg.from.x) * msg.progress;
    const y = msg.from.y + (msg.to.y - msg.from.y) * msg.progress;
    this.ctx.beginPath();
    this.ctx.arc(x, y, 12, 0, Math.PI * 2);
    this.ctx.fillStyle = MessageColors[msg.type] || "#999";
    this.ctx.fill();
    this.ctx.strokeStyle = "white";
    this.ctx.lineWidth = 1.5;
    this.ctx.stroke();
    this.ctx.fillStyle = "white";
    this.ctx.font = "bold 9px Arial";
    this.ctx.textAlign = "center";
    this.ctx.textBaseline = "middle";
    const typeText =
      {
        [MessageType.PREPARE]: "P",
        [MessageType.PROPOSE]: "PO",
        [MessageType.PROMISE]: "PR",
        [MessageType.ACCEPT]: "A",
      }[msg.type] || "";
    this.ctx.fillText(typeText, x, y);
  }

  /**
   * Starts polling server state
   */
  startStatePolling() {
    if (this.pollingInterval) clearInterval(this.pollingInterval);
    console.log("Starting state polling with delay:", this.pollingDelay);
    this.pollingInterval = setInterval(() => {
      console.log("Polling server state...");
      fetch(`${this.apiBaseUrl}/state`)
        .then((response) => response.json())
        .then((data) => {
          console.log("Received server state:", data);
          this.updateFromServerState(data);
        })
        .catch((error) => {
          console.error("Error polling state:", error);
          this.addLog("Error polling state", "network");
        });
    }, this.pollingDelay);
  }

  /**
   * Updates UI elements
   */
  updateUI() {
    console.log("Updating UI with stats:", this.stats);
    console.log("Current phase:", this.currentPhase);
    
    const phaseElement = document.getElementById("current-phase");
    if (phaseElement) {
      phaseElement.textContent = this.currentPhase;
    }
    
    const prepareElement = document.getElementById("prepare-count");
    if (prepareElement) {
      prepareElement.textContent = this.stats.prepareCount;
    }
    
    const promiseElement = document.getElementById("promise-count");
    if (promiseElement) {
      promiseElement.textContent = this.stats.promiseCount;
    }
    
    const proposalElement = document.getElementById("proposal-count");
    if (proposalElement) {
      proposalElement.textContent = this.stats.proposalCount;
    }
    
    const acceptElement = document.getElementById("accept-count");
    if (acceptElement) {
      acceptElement.textContent = this.stats.acceptCount;
    }
  }
}

document.addEventListener("DOMContentLoaded", () => {
  console.log("DOM loaded, initializing PaxosVisualizer");
  window.paxosVisualizer = new PaxosVisualizer();
});

// Fallback initialization
if (
  document.readyState === "complete" ||
  document.readyState === "interactive"
) {
  setTimeout(() => {
    if (!window.paxosVisualizer) {
      console.log("Fallback initialization");
      window.paxosVisualizer = new PaxosVisualizer();
    }
  }, 100);
}
