$(document).ready(function () {

    setInterval(updateMessages, 1000);
    setInterval(updateNodes, 1000);
    setInterval(updateRoutes, 1000);
    setInterval(getRound, 1000);

    getID();


    let messageForm = "messageForm";
    let messageText = "messageText";
    document.getElementById(messageForm).addEventListener("submit", sendMessage);

    function sendMessage() {
        event.preventDefault();
        let message = document.getElementById(messageText).value;
        $.ajax({
            type: "POST",
            url: "message",
            data: JSON.stringify({message: message}),
            contentType: "application/json",
            success: function () {
                updateMessages();
            }
        });
    }

    let receivedMessages = "receivedMessages";
    let receivedPrivate = "receivedPrivate";
    updateMessages();

    //get new messages
    function updateMessages() {
        var messages = [];
        var privateMessages = [];
        let chatBox = document.getElementById(receivedMessages);
        let privateChatBox = document.getElementById(receivedPrivate);
        $.getJSON("message", function (data) {
            // Rumor messages
            if (data["Rumor"] !== undefined && data["Rumor"] !== null) {
                Object.entries(data["Rumor"]).forEach(([key]) => {
                    messages.push(data["Rumor"][key]);
                });
            }


            //Object.entries(data).sort(function (a, b) {
            //    return a.MessageNumber - b.MessageNumber;
            //});
            chatBox.value = "Welcome to the chat! \n";
            for (let key in messages) {
                chatBox.value += messages[key]["Rumor"]["Origin"] + ":" + messages[key]["Rumor"]["Text"] + "\n";
            }

            // Private messages
            if (privateID !== "") {
                if (Object.entries(data["Private"][privateID] == null)) {
                    privateChatBox.value = "Chatting with: " + privateID + "\n";
                }
                Object.entries(data["Private"][privateID]).forEach(([key]) => {
                    privateMessages.push(data["Private"][privateID][key]);
                });
                privateChatBox.value = "Chatting with: " + privateID + "\n";
                console.log(privateMessages);
                for (let message in privateMessages) {
                    privateChatBox.value += privateMessages[message]["Origin"] + ":" + privateMessages[message]["Text"] + "\n";
                }
            }
        })
    }

    let peerForm = "addPeerForm";
    let peerText = "newPeerArea";
    document.getElementById(peerForm).addEventListener("submit", addPeer);

    function addPeer() {
        let peer = document.getElementById(peerText).value;
        $.ajax({
            type: "POST",
            url: "node",
            data: JSON.stringify({"peerID": peer}),
            contentType: "application/json",
            success: function () {
                updateNodes();
            }
        });
    }

    let newPeers = "knownPeers";
    updateNodes();

    //update the nodes screen
    function updateNodes() {
        let peersArea = document.getElementById(newPeers);
        $.getJSON("node", function (data) {
            peersArea.value = "";
            for (var key in data) {
                peersArea.value += data[key] + "\n";
            }
        })

    }

    updateRoutes();

    function updateRoutes() {
        let oldRouteTable = document.getElementById("routing_table").tBodies[0];
        $.getJSON("route", function (data) {
            var newRouteTable = document.createElement('tbody');

            Object.entries(data).forEach(([key, value]) => {
                var row = newRouteTable.insertRow(newRouteTable.rows.length);
                var origin = row.insertCell(0);
                var nextHop = row.insertCell(1);
                var btn = document.createElement("BUTTON");   // Create a <button> element
                btn.innerHTML = key; // Insert text
                btn.addEventListener("click", function () {
                    showTextBox(key);
                });
                origin.parentNode.replaceChild(btn, origin);
                let host = Object.keys(value)[0];
                nextHop.innerText = host + ":" + value[host];
            });
            if (oldRouteTable.parentNode != null) {
                oldRouteTable.parentNode.replaceChild(newRouteTable, oldRouteTable);
            }

        })
    }

    let privateMessageForm = "privateMessageForm";
    let privateMessageText = "privateMessageText";
    let privateMessageID = "privateMessageID";

    let privateID = "";

    function showTextBox(ID) {
        document.getElementById(privateMessageID).innerText = ID;
        document.getElementById(privateMessageForm).style.display = "block";
        privateID = ID;
        updateMessages();
    }

    document.getElementById(privateMessageForm).addEventListener("submit", sendPrivateMessage);

    function sendPrivateMessage() {
        let message = document.getElementById(privateMessageText).value;
        let ID = document.getElementById(privateMessageID).innerText;
        event.preventDefault();
        $.ajax({
            type: "POST",
            url: "message",
            data: JSON.stringify({message: message, destination: ID}),
            contentType: "application/json",
            success: function () {
                updateMessages();
            }
        });
    }


    function getID() {
        $.getJSON("id", function (data) {
            document.getElementById("ID_box").value = data.name;
        })
    }

    let shareFileForm = "shareFileForm";
    document.getElementById(shareFileForm).addEventListener("submit", shareFile);

    function shareFile() {
        let file = document.getElementById("fileID").value;
        file = file.replace(/^.*\\/, "");
        $.ajax({
            type: "POST",
            url: "share",
            data: JSON.stringify({file: file}),
            contentType: "application/json",
            success: function () {
                //updateMessages();
            }
        });
    }

    let downloadFileForm = "downloadFileForm";
    document.getElementById(downloadFileForm).addEventListener("submit", downloadFile);

    function downloadFile() {
        let destination = document.getElementById("destination").value;
        let hash = document.getElementById("hash").value;
        let fileName = document.getElementById("fileName").value;

        $.ajax({
            type: "POST",
            url: "download",
            data: JSON.stringify({destination: destination, hash: hash, fileName: fileName}),
            contentType: "application/json",
            success: function () {
                //updateMessages();
                alert("Success")
            }
        });

    }

    let searchFileForm = "searchFileForm";
    let searchResults = "searchResults";
    document.getElementById(searchFileForm).addEventListener("submit", searchFile);

    setInterval(updateFiles, 1000);

    function searchFile() {
        let keywords = document.getElementById("keywords").value;
        let budget = document.getElementById("budget").value;
        document.getElementById(searchResults).style.display = "block";

        $.ajax({
            type: "POST",
            url: "search",
            data: JSON.stringify({keywords: keywords, budget: budget}),
            contentType: "application/json",
            success: function () {
                setInterval(updateFiles, 1000);
                alert("Success")
            }
        });

    }

    function updateFiles() {
        let oldFileTable = document.getElementById("fileList").tBodies[0];
        $.getJSON("search", function (data) {
            var newFileTable = document.createElement('tbody');
            Object.entries(data).forEach(([fileName, _]) => {
                var row = newFileTable.insertRow(newFileTable.rows.length);
                var name = row.insertCell(0);
                var hash = row.insertCell(1);
                name.innerHTML = data[fileName]["Name"];
                hash.innerHTML = data[fileName]["MetaFileHash"];
                name.addEventListener("dblclick", function () {
                    downloadSearchFile(name.innerHTML, hash.innerHTML);
                });
            });
            if (oldFileTable.parentNode != null) {
                oldFileTable.parentNode.replaceChild(newFileTable, oldFileTable);
            }
        })
    }

    function downloadSearchFile(fileName, hash) {
        $.ajax({
            type: "POST",
            url: "download",
            data: JSON.stringify({destination: "", hash: hash, fileName: fileName}),
            contentType: "application/json",
            success: function () {
                alert("Downloaded file successfully")
            }
        });
    }

    function getRound() {
        $.getJSON("round", function (data) {
            document.getElementById("round").value = data.round;
        })
    }


});



