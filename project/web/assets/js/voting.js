$(document).ready(function () {
    let nodeIdEl = $("#node-id");
    let pollsEl = $("#polls");
    let questionEl = $("#add-poll-question");
    let votersEl = $("#add-poll-voters");
    let addButtonEl = $("#add-poll-button");

    $.getJSON("../id", function (data) {
        nodeIdEl.html("<p>" + data.id + "</p>");
    });


    let pollIdsSet = new Set();
    let pollsList = [];

    function constructPollHtml(poll) {
        htmlStr = "ID " + poll.id + " QUESTION " + poll.question + " FROM " + poll.origin;
        if (poll.canVote) {
            htmlStr += " <select id='select-vote-" + poll.id + "'><option value='0'>No</option><option value='1'>Yes</option></select>" +
                " <button type='button' class='button-vote' id='" + poll.id + "'>Vote</button>";

        }
        if (poll.canCount) {
            htmlStr += " <button type='button' class='button-count' id='" + poll.id + "'>Count Votes</button>";
        }
        if (poll.result.count >= 0) {
            htmlStr += " RESULT " + poll.result.count + " (" + poll.result.timestamp + ")"
        }
        return htmlStr;
    }

    function setClicks() {
        $(".button-vote").unbind("click");
        $(".button-count").unbind("click");

        $(".button-vote").click(function () {
            pollId = $(this).attr("id");
            console.log("Voting for  " + pollId);
            vote = $("#select-vote-" + pollId).val();
            $.ajax({
                type: 'POST',
                url: 'poll/' + pollId + "/vote",
                data: JSON.stringify({"vote": vote}),
                contentType: "application/json",
                dataType: 'json'
            });
        });
        $(".button-count").click(function () {
            pollId = $(this).attr("id");
            console.log("Counting " + pollId);
            $.ajax({
                type: "POST",
                url: "poll/" + pollId + "/count",
            })
        });
    }

    function refreshPolls() {
        $.getJSON("polls", function (data) {
            updatedPollIds = new Map();
            for (i = 0; i < data.polls.length; i++) {
                updatedPollIds.set(data.polls[i].id, data.polls[i])
            }

            for (i = 0; i < pollsList.length; i++) {
                if (!updatedPollIds.has(pollsList[i].id)) {
                    pollsEl.find("li")[i].remove();
                    console.log("Removing " + i)
                }
            }

            pollsList = pollsList.filter(function (poll) {
                return updatedPollIds.has(poll.id);
            });

            // Add the new elements from the updated polls
            for (i = 0; i < data.polls.length; i++) {
                poll = data.polls[i];
                if (!pollIdsSet.has(poll.id)) {
                    pollsList.push(poll);
                    console.log("Adding pollid " + poll.id);

                    htmlStr = constructPollHtml(poll);
                    pollsEl.append("<li>" + htmlStr + "</li>");
                    setClicks();
                }
            }
            pollIdsSet = new Set(pollsList.map(function (poll) {
                return poll.id;
            }));

            for (i = 0; i < pollsList.length; i++) {
                poll1 = pollsList[i];
                poll2 = updatedPollIds.get(pollsList[i].id);
                if (poll1.canCount != poll2.canCount || poll1.canVote != poll2.canVote || poll1.result.count != poll2.result.count) {
                    $("#polls li:nth-child(" + (i + 1) + ")").html(constructPollHtml(updatedPollIds.get(pollsList[i].id)));
                    console.log("Updating html of " + i);
                    console.log(JSON.stringify(poll1));
                    console.log(JSON.stringify(poll2));
                    pollsList[i] = poll2;
                    setClicks();
                }
            }


        }).always(function () {
            setTimeout(refreshPolls, 100);
        });
    }

    function getBlocks() {
        let oldBlockTable = document.getElementById("blockchainTable").tBodies[0];
        $.getJSON("blockchain", function (data) {
            console.log(data);
            var newBlockTable = document.createElement('tbody');
            Object.entries(data["blocks"]).forEach(([index, block]) => {
                var row = newBlockTable.insertRow(newBlockTable.rows.length);
                var blockNumber = row.insertCell(0);
                var timestamp = row.insertCell(1);
                var origin = row.insertCell(2);
                var hash = row.insertCell(3);
                var prevHash = row.insertCell(4);
                var difficulty = row.insertCell(5);
                blockNumber.innerHTML = index;
                timestamp.innerHTML = block.timestamp;
                origin.innerHTML = block.origin;
                hash.innerHTML = block.hash;
                prevHash.innerHTML = block.prevHash;
                difficulty.innerHTML = block.difficulty;
            });
            if (oldBlockTable.parentNode != null) {
                oldBlockTable.parentNode.replaceChild(newBlockTable, oldBlockTable);
            }

        }).always(function () {
            setTimeout(refreshPolls, 100);
        });
    }

    addButtonEl.click(function () {
        $.ajax({
            type: 'POST',
            url: 'polls',
            data: JSON.stringify({"question": questionEl.val(), "voters": votersEl.val()}),
            contentType: "application/json",
            dataType: 'json'
        });
    });

    refreshPolls();
    getBlocks();
});