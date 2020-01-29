$(document).ready(function(){
    let nodeIdEl = $("#node-id");
    let pollsEl = $("#polls");

    $.getJSON("../id", function(data) {
        nodeIdEl.html("<p>" + data.id + "</p>");
    });


    let pollsSet = new Set();
    let pollsList = [];
    function refreshPolls(){
        $.getJSON("polls", function(data) {
            // Remove the elements that dont appear in the updated polls
            updatedPolls = new Set(data.polls);
            pollsList = pollsList.filter(function(poll) {
                return updatedPolls.has(JSON.stringify(poll));
            });

            // Add the new elements from the updated polls
            for (i = 0; i < data.polls.length; i++) {
                poll = data.polls[i];
                if (!pollsSet.has(JSON.stringify(poll))) {
                    pollsList.push(poll);
                }
            }

            pollsSet = new Set(pollsList.map(function(poll) {
                return JSON.stringify(poll);
            }));

            // Now update the html element
            pollsHtml = "";
            for (i = 0; i < pollsList.length; i++) {
                pollsHtml += "<li>" + "ID " + pollsList[i].id +  " QUESTION " + pollsList[i].question + " FROM " + pollsList[i].origin + "</li>";
            }
            pollsEl.html(pollsHtml);

        }).always(function(){
            setTimeout(refreshPolls, 100);
        });
    }

    refreshPolls();
});