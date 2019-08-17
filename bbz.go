package main

import (
	"github.com/gorilla/websocket"
	"log"
	"math/rand"
	"net/http"
	"time"
)

func newId(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

type Event struct {
	from string
	name string
	data *string
}

type Node struct {
	id             string
	nodeController *NodeController
	group          *Group
	conn           *websocket.Conn
}

func newNode(conn *websocket.Conn, nc *NodeController, group *Group) *Node {
	id := newId(32)
	for group.nodes[id] != nil {
		id = newId(32)
	}
	node := &Node{
		id:             id,
		nodeController: nc,
		group:          group,
		conn:           conn,
	}
	group.nodes[id] = node
	group.data[id] = nil
	go node.run()
	log.Print("New node")
	return node
}

func (node *Node) run() {
	defer func() {
		node.nodeController.unregister <- node
	}()

	for {
		message := make(map[string]string)
		err := node.conn.ReadJSON(&message)
		if err != nil {
			break
		}
		data := message["data"]
		event := &Event{
			from: node.id,
			name: message["event"],
			data: &data,
		}
		node.group.events <- event
	}
}

type Group struct {
	nodes  map[string]*Node
	data   map[string]*string
	events chan *Event
}

func newGroup() *Group {
	g := &Group{
		nodes:  make(map[string]*Node),
		data:   make(map[string]*string),
		events: make(chan *Event),
	}
	go g.run()
	log.Print("New group")
	return g
}

func (group *Group) onReady() {
	for id, node := range group.nodes {
		ready := make(map[string]string)
		ready["event"] = "ready"
		ready["name"] = id
		for op, _ := range group.nodes {
			if id != op {
				ready["opponent"] = op
				break
			}
		}
		node.conn.WriteJSON(ready)
	}
}

func (group *Group) onSync(from string, data string) {
	group.data[from] = &data
	syncAll := true
	for _, data := range group.data {
		if data == nil {
			syncAll = false
		}
	}
	if !syncAll {
		return
	}
	sync := make(map[string]string)
	sync["event"] = "sync"
	for id, data := range group.data {
		sync[id] = *data
	}
	for _, node := range group.nodes {
		node.conn.WriteJSON(sync)
	}
	for id, _ := range group.nodes {
		group.data[id] = nil
	}
}

func (group *Group) onBroadcast(from string, message string) {
	broadcast := make(map[string]string)
	broadcast["event"] = "broadcast"
	broadcast["from"] = from
	broadcast["message"] = message
	for _, node := range group.nodes {
		node.conn.WriteJSON(broadcast)
	}
}

func (group *Group) onEvent(event *Event) {
	if event.name == "ready" {
		group.onReady()
	}
	if event.name == "sync" {
		group.onSync(event.from, *event.data)
	}
	if event.name == "broadcast" {
		group.onBroadcast(event.from, *event.data)
	}
}

func (group *Group) run() {
	for event := range group.events {
		group.onEvent(event)
	}
}

type NodeController struct {
	currentGroup *Group
	groups       map[*Group]bool
	register     chan *websocket.Conn
	unregister   chan *Node
}

func newNodeController() *NodeController {
	nc := &NodeController{
		currentGroup: nil,
		groups:       make(map[*Group]bool),
		register:     make(chan *websocket.Conn),
		unregister:   make(chan *Node),
	}
	nc.currentGroup = newGroup()
	go nc.run()
	return nc
}

func (nc *NodeController) run() {
	rand.Seed(time.Now().UTC().UnixNano())
	for {
		select {
		case conn := <-nc.register:
			newNode(conn, nc, nc.currentGroup)
			if len(nc.currentGroup.nodes) >= 2 {
				nc.groups[nc.currentGroup] = true
				nc.currentGroup.events <- &Event{
					from: "master",
					name: "ready",
					data: nil,
				}
				nc.currentGroup = newGroup()
			}
		case node := <-nc.unregister:
			log.Print("Delete node")
			node.conn.Close()
			delete(node.group.nodes, node.id)
			for _, other := range node.group.nodes {
				other.conn.Close()
			}
			if len(node.group.nodes) == 0 {
				log.Print("Delete group")
				close(node.group.events)
				delete(nc.groups, node.group)
				if node.group == nc.currentGroup {
					nc.currentGroup = newGroup()
				}
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func serveWs(nc *NodeController, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	nc.register <- conn
}

func main() {
	nodeController := newNodeController()
	http.HandleFunc("/ws/bbz", func(w http.ResponseWriter, r *http.Request) {
		serveWs(nodeController, w, r)
	})
	log.Print("Server running on 127.0.0.1:13001")
	log.Fatal(http.ListenAndServe("127.0.0.1:13001", nil))
}
