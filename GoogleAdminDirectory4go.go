package googleadmindirectory4go

import (
	"context"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

var ctx = context.Background()

func Initialize(client *http.Client, adminEmail string) *GoogleDirectory {
	service, err := admin.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}

	log.Printf("Initialized GoogleAdmin4Go as (%s)\n", adminEmail)
	return &GoogleDirectory{Service: service, AdminEmail: adminEmail}
}

type GoogleDirectory struct {
	Service    *admin.Service
	AdminEmail string
}

/*Users*/
func (receiver *GoogleDirectory) GetUsers(query string) []*admin.User {
	request := receiver.Service.Users.List().Fields("*").Domain("usaid.gov").Query(query).MaxResults(500)
	var userList []*admin.User
	for {
		response, err := request.Do()
		if err != nil {
			log.Println(err.Error())
			panic(err)
		}
		userList = append(userList, response.Users...)
		log.Printf("Query \"%s\" returned %d users thus far.\n", query, len(userList))
		if response.NextPageToken == "" {
			break
		}
		request.PageToken(response.NextPageToken)
	}

	return userList
}

func (receiver *GoogleDirectory) GetGroupsByUser(userEmail string) map[*admin.Group]*admin.Member {
	groupList := receiver.GetGroups("memberKey=" + userEmail)
	groupMap := make(map[*admin.Group]*admin.Member)
	for counter, group := range groupList {
		memberResponse, err := receiver.Service.Members.Get(group.Email, userEmail).Fields("*").Do()
		if err != nil {
			log.Println(err.Error())
			panic(err)
		}
		log.Printf("(%s) Group [%d] of [%d] {%s}: %s <%s>\n", userEmail, counter, len(groupList), memberResponse.Role, group.Name, group.Email)
		groupMap[group] = memberResponse
	}
	return groupMap
}

/*Groups*/
func (receiver *GoogleDirectory) GetGroups(query string) []*admin.Group {
	request := receiver.Service.Groups.List().Domain("usaid.gov").Fields("*")
	if query != "" {
		request.Query(query)
	}
	var groupList []*admin.Group
	for {
		response, err := request.Do()
		if err != nil {
			log.Println(err.Error())
			panic(err)
		}
		groupList = append(groupList, response.Groups...)
		log.Printf("Query \"%s\" returned %d groups thus far.\n", query, len(groupList))
		if response.NextPageToken == "" {
			break
		}
		request.PageToken(response.NextPageToken)
	}
	return groupList
}

func (receiver *GoogleDirectory) GetGroupByEmail(groupEmail string) *admin.Group {
	response, err := receiver.Service.Groups.Get(groupEmail).Fields("*").Do()
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	return response
}

/*Members*/
func (receiver *GoogleDirectory) InsertMember(groupEmail string, member *admin.Member, wg *sync.WaitGroup) {
	request := receiver.Service.Members.Insert(groupEmail, member)
	_, err := request.Do()
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			log.Println(err.Error() + " - Skipping")
			wg.Done()
			return
		}
		log.Println(err)
		log.Printf("Insertion of [%s] to group (%s) failed... Retrying in 2 seconds.", member.Email, groupEmail)
		time.Sleep(2 * time.Second)
		receiver.InsertMember(groupEmail, member, wg)
		return
	}
	log.Printf("Insertion of [%s] to (%s) was successful!", member.Email, groupEmail)
	wg.Done()
}

func (receiver *GoogleDirectory) InsertMembers(memberList []*admin.Member, groupEmail string, maxRoutines int) {
	totalInserts := len(memberList)
	insertCounter := 0
	log.Printf("Total members to insert into from %s: %d\n", groupEmail, totalInserts)

	for {
		if len(memberList) <= maxRoutines {
			maxRoutines = len(memberList)
		}
		wg := &sync.WaitGroup{}
		wg.Add(maxRoutines)
		for i := range memberList[:maxRoutines] {
			log.Printf("Insert user  [%d] of [%d]\n", insertCounter, totalInserts)
			insertCounter++
			memberToInsert := memberList[i]
			go receiver.InsertMember(groupEmail, memberToInsert, wg)
		}
		wg.Wait()

		memberList = memberList[maxRoutines:]
		if len(memberList) == 0 {
			break
		}
	}
	log.Printf("Total members inserted into %s: %d\n", groupEmail, insertCounter)

}

func (receiver *GoogleDirectory) DeleteMember(groupEmail, memberEmail string, wg *sync.WaitGroup) {
	request := receiver.Service.Members.Delete(groupEmail, memberEmail)
	err := request.Do()
	if err != nil {
		log.Println(err)
		log.Printf("Deletion of [%s] from group (%s) failed... Retrying in 2 seconds", memberEmail, groupEmail)
		time.Sleep(2 * time.Second)
		receiver.DeleteMember(groupEmail, memberEmail, wg)
		return
	}
	log.Printf("Deletetion of [%s] from (%s) was successful!", memberEmail, groupEmail)
	wg.Done()
}

func (receiver *GoogleDirectory) DeleteMembers(deleteList []string, groupEmail string, maxRoutines int) {
	totalDeletes := len(deleteList)
	deleteCounter := 0
	log.Printf("Total members to remove from %s: %d\n", groupEmail, totalDeletes)

	for {
		if len(deleteList) <= maxRoutines {
			maxRoutines = len(deleteList)
		}
		wg := &sync.WaitGroup{}
		wg.Add(maxRoutines)
		for i := range deleteList[:maxRoutines] {
			log.Printf("Delete user  [%d] of [%d]\n", deleteCounter, totalDeletes)
			deleteCounter++
			memberToDelete := deleteList[i]
			receiver.DeleteMember(groupEmail, memberToDelete, wg)
		}
		wg.Wait()

		deleteList = deleteList[maxRoutines:]
		if len(deleteList) == 0 {
			break
		}
	}

	log.Printf("Total members removed from %s: %d\n", groupEmail, deleteCounter)
}

func (receiver *GoogleDirectory) GetAllMembersFromGroup() {

}
