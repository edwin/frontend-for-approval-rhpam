package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/login", login)
	http.HandleFunc("/welcome", welcome)
	http.HandleFunc("/save", save)
	http.HandleFunc("/get-list-approvals", getListApprovals)
	http.HandleFunc("/get-approval", getApproval)
	http.HandleFunc("/approve", doApproving)
	http.HandleFunc("/logout", logout)
	http.ListenAndServe(":8081", nil)
}

func login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	redirectTarget := "/"

	if (username == "adminUser" || username == "spv01") && password == "password" {
		setSession(username, password, w)
		redirectTarget = fmt.Sprintf("/welcome?%d", int32(time.Now().Unix()))
	}
	http.Redirect(w, r, redirectTarget, 302)

	return
}

func logout(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     "login_cookie",
		Value:    "",
		Path:     "/",
		Expires: time.Unix(0, 0),

		HttpOnly: true,
	}

	http.SetCookie(w, cookie)

	redirectTarget := "/"
	http.Redirect(w, r, redirectTarget, 302)

	return
}

func welcome(w http.ResponseWriter, r *http.Request) {
	// set header response
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("if_modified_since", "off")
	w.Header().Set("Last-Modified", "")

	sessionToken := getUsernameAndPassword(w, r)

	if strings.Contains(sessionToken, "adminUser") {
		http.ServeFile(w, r, "./static/entry.html")
		return
	} else if strings.Contains(sessionToken, "spv01") {
		http.ServeFile(w, r, "./static/approval.html")
		return
	}
}

// send to pam
func save(w http.ResponseWriter, r *http.Request) {
	// get username and password
	value := getUsernameAndPassword(w, r)

	// capture parameters
	days := r.FormValue("days")
	purpose := r.FormValue("purpose")
	id := r.FormValue("id") // this is correlation-id

	// create new instances
	reader := strings.NewReader(fmt.Sprintf(
		"{\"application\": "+
					"{"+
						"\"com.myspace.approval_system.Request\": {"+
							"\"days\": %s,"+
							"\"purpose\":\"%s\""+
							"}"+
					"}"+
				"}",
		days,
		purpose))

	request, _ := http.NewRequest("POST", "http://"+value+"@localhost:8080/kie-server/services/rest/server/" +
		"containers/approval_system_1.0.1-SNAPSHOT/processes/approval/instances/correlation/"+id, reader)

	// set header
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	// fire to pam
	client := &http.Client{}
	resp, err := client.Do(request)

	if err != nil {
		fmt.Println(err)
	}

	// close http
	defer resp.Body.Close()

	// capture the response
	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Fprintln(os.Stdout, "response from pam : ", string(body))

	http.ServeFile(w, r, "./static/success.html")
}

func getListApprovals(w http.ResponseWriter, r *http.Request) {
	// get username and password
	value := getUsernameAndPassword(w, r)

	request, _ := http.NewRequest("GET", "http://"+value+"@localhost:8080/kie-server/services/rest/server/queries/tasks/instances/owners?page=0&pageSize=10&sortOrder=true&sort=taskId", nil)

	// set request header
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	// set header response
	w.Header().Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(request)

	if err != nil {
		fmt.Println(err)
		fmt.Fprint(w, "{\"message\":\"fail\"}")
		return
	}

	defer resp.Body.Close()

	// get response from pam
	body, _ := ioutil.ReadAll(resp.Body)

	fmt.Fprint(w, string(body))
	return
}

func getApproval(w http.ResponseWriter, r *http.Request) {
	// get username and password
	value := getUsernameAndPassword(w, r)

	// construct url request to PAM
	id := r.URL.Query()["id"]
	request, _ := http.NewRequest("GET", fmt.Sprintf("http://"+value+"@localhost:8080/kie-server/services/rest/server/containers/approval_system_1.0.1-SNAPSHOT/processes/instances/%s/variables", id[0]), nil)

	// set request header
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	// set header response
	w.Header().Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(request)

	if err != nil {
		fmt.Println(err)
		fmt.Fprint(w, "{\"message\":\"fail\"}")
		return
	}

	defer resp.Body.Close()

	// get response from pam
	body, _ := ioutil.ReadAll(resp.Body)

	fmt.Fprint(w, string(body))

	return
}

func doApproving(w http.ResponseWriter, r *http.Request) {
	// get username and password
	value := getUsernameAndPassword(w, r)

	// create new instances
	id := r.URL.Query()["id"][0]
	status := r.URL.Query()["status"][0]
	reader := strings.NewReader(fmt.Sprintf(
		"{\"approved\": %s }",
			status))

	// url for start task
	request, _ := http.NewRequest("PUT", fmt.Sprintf("http://"+value+"@localhost:8080/kie-server/services/rest/server/containers/approval_system_1.0.1-SNAPSHOT/tasks/%s/states/started", id), nil)

	// set header response
	w.Header().Set("Content-Type", "application/json")

	// start the task
	client := &http.Client{}
	resp, err := client.Do(request)

	// url for completing the task
	request, _ = http.NewRequest("PUT", fmt.Sprintf("http://"+value+"@localhost:8080/kie-server/services/rest/server/containers/approval_system_1.0.1-SNAPSHOT/tasks/%s/states/completed", id), reader)

	// set request header
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	// complete the task
	resp, err = client.Do(request)

	if err != nil {
		fmt.Println(err)
		fmt.Fprint(w, "{\"message\":\"fail\"}")
		return
	}

	defer resp.Body.Close()

	// get response from pam
	body, _ := ioutil.ReadAll(resp.Body)

	fmt.Fprint(w, string(body))

	return
}

func setSession(userName string, password string, response http.ResponseWriter) {
	cookie := &http.Cookie{
		Name: "login_cookie",
		Value: userName+":"+password,
		Path: "/",
		MaxAge: 86400,
	}

	http.SetCookie(response, cookie)
}

func getUsernameAndPassword(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("login_cookie")

	if err != nil {
		if err == http.ErrNoCookie {
			http.Redirect(w, r, "/", 302)
			return ""
		}
		// For any other type of error, return a bad request status
		w.WriteHeader(http.StatusBadRequest)
		return ""
	}

	// get username and password
	return cookie.Value
}