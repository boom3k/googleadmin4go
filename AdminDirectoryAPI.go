package googleadmin4go

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

func BuildNewDirectoryAPI(client *http.Client, adminEmail string, ctx *context.Context) *DirectoryAPI {
	newDirectoryAPI := &DirectoryAPI{}
	newDirectoryAPI.Build(client, adminEmail, ctx)
	return newDirectoryAPI
}

func (receiver *DirectoryAPI) Build(client *http.Client, adminEmail string, ctx *context.Context) *DirectoryAPI {
	service, err := admin.NewService(*ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	response, err := service.Users.Get(adminEmail).Fields("customerId").Do()
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	receiver.Service = service
	receiver.CustomerID = response.CustomerId
	receiver.AdminEmail = adminEmail
	receiver.Domain = strings.Split(adminEmail, "@")[1]
	log.Printf("DirectoryAPI --> \n"+
		"\tService: %v\n"+
		"\tCustomerID: %s\n"+
		"\tAdminEmail: %s\n"+
		"\tDomain: %s\n", &receiver.Service, receiver.CustomerID, receiver.AdminEmail, receiver.Domain,
	)
	return receiver
}

type DirectoryAPI struct {
	Service    *admin.Service
	CustomerID string
	AdminEmail string
	Domain     string
}

/*Users*/
func (receiver *DirectoryAPI) GetUsers(query string, ch chan []*admin.User) []*admin.User {
	request := receiver.Service.Users.List().Fields("*").Domain(receiver.Domain).Query(query).MaxResults(500)
	var userList []*admin.User
	for {
		response, err := request.Do()
		if err != nil {
			log.Println(err.Error())
			panic(err)
		}
		userList = append(userList, response.Users...)

		log.Printf("<- %v[%d] sent by %v.GetUsers()\n", ch, len(response.Users), receiver)
		ch <- response.Users
		log.Printf("%v.GetUsers() waiting for %v[nil] to come back <-\n", receiver, ch)
		<-ch

		log.Printf("Query \"%s\" returned %d users thus far.\n", query, len(userList))
		if response.NextPageToken == "" {
			close(ch)
			log.Printf("<- Closed channel %v per Sender %v.GetUsers()\n", ch, receiver)
			break
		}
		request.PageToken(response.NextPageToken)
	}
	return userList
}

func (receiver *DirectoryAPI) GetGroupsByUser(userEmail string) map[*admin.Group]*admin.Member {
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
func (receiver *DirectoryAPI) GetGroups(query string) []*admin.Group {
	request := receiver.Service.Groups.List().Domain(receiver.Domain).Fields("*")
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

func (receiver *DirectoryAPI) GetGroupByEmail(groupEmail string) *admin.Group {
	response, err := receiver.Service.Groups.Get(groupEmail).Fields("*").Do()
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	return response
}

/*Members*/
func (receiver *DirectoryAPI) PushMemberByEmail(groupEmail, userEmail, role string) *admin.Member {
	return receiver.PushMember(groupEmail, &admin.Member{Email: userEmail, Role: role})
}

func (receiver *DirectoryAPI) PushMember(groupEmail string, member *admin.Member) *admin.Member {
	request := receiver.Service.Members.Insert(groupEmail, member)
	result, err := request.Do()
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			log.Println(err.Error() + " - Skipping")
			return nil
		}
		log.Println(err)
		log.Printf("Insertion of [%s] to group (%s) failed... Retrying in 2 seconds.", member.Email, groupEmail)
		time.Sleep(2 * time.Second)
		receiver.PushMember(groupEmail, member)
		return nil
	}
	log.Printf("Insertion of [%s] to (%s) was successful!", member.Email, groupEmail)
	return result
}

func (receiver *DirectoryAPI) InsertMembers(memberList []*admin.Member, groupEmail string, maxRoutines int) []*admin.Member {
	totalInserts := len(memberList)
	var completedInserts []*admin.Member
	log.Printf("Total members to insert into from %s: %d\n", groupEmail, totalInserts)

	for {
		if len(memberList) <= maxRoutines {
			maxRoutines = len(memberList)
		}
		wg := &sync.WaitGroup{}
		wg.Add(maxRoutines)
		for i := range memberList[:maxRoutines] {
			log.Printf("PushMemberByEmail user  [%d] of [%d]\n", len(completedInserts), totalInserts)
			memberToInsert := memberList[i]
			go func() {
				receiver.PushMember(groupEmail, memberToInsert)
				completedInserts = append(completedInserts, memberToInsert)
				wg.Done()
			}()
			//go receiver.PushMemberWorker(groupEmail, memberToInsert, wg)
		}
		wg.Wait()

		memberList = memberList[maxRoutines:]
		if len(memberList) == 0 {
			break
		}
	}
	log.Printf("Total members inserted into %s: %d\n", groupEmail, len(completedInserts))
	return completedInserts
}

func (receiver *DirectoryAPI) DeleteMember(groupEmail, memberEmail string) {
	request := receiver.Service.Members.Delete(groupEmail, memberEmail)
	err := request.Do()
	if err != nil {
		log.Println(err)
		log.Printf("Deletion of [%s] from group (%s) failed... Retrying in 2 seconds", memberEmail, groupEmail)
		time.Sleep(2 * time.Second)
		receiver.DeleteMember(groupEmail, memberEmail)
		return
	}
	log.Printf("Deletetion of [%s] from (%s) was successful!", memberEmail, groupEmail)
}

func (receiver *DirectoryAPI) DeleteMembers(deleteList []string, groupEmail string, batchSize int) {
	totalDeletes := len(deleteList)
	deleteCounter := 0
	log.Printf("Total members to remove from %s: %d\n", groupEmail, totalDeletes)

	for {
		if len(deleteList) <= batchSize {
			batchSize = len(deleteList)
		}
		wg := &sync.WaitGroup{}
		wg.Add(batchSize)
		for i := range deleteList[:batchSize] {
			log.Printf("Delete user  [%d] of [%d]\n", deleteCounter, totalDeletes)
			deleteCounter++
			memberToDelete := deleteList[i]
			go func() {
				receiver.DeleteMember(groupEmail, memberToDelete)
				wg.Done()
			}()
		}
		wg.Wait()

		deleteList = deleteList[batchSize:]
		if len(deleteList) == 0 {
			break
		}
	}

	log.Printf("Total members removed from %s: %d\n", groupEmail, deleteCounter)
}

func (receiver *DirectoryAPI) GetMembers(groupEmail string, roles []string) []*admin.Member {
	allRoles := strings.ToUpper(strings.Join(roles, ","))
	log.Printf("Retreiving  %s members from %s\n", allRoles, groupEmail)
	var members []*admin.Member
	for {
		request, err := receiver.Service.Members.List(groupEmail).Roles(allRoles).Fields("*").MaxResults(200).Do()
		if err != nil {
			log.Println(err.Error())
			if strings.Contains(err.Error(), "Quota") {
				log.Println("Backing off for 3 seconds...")
				time.Sleep(time.Second * 3)
				return receiver.GetMembers(groupEmail, roles)
			}
			return nil
		}
		members = append(members, request.Members...)
		nextPageToken := request.NextPageToken
		if nextPageToken == "" {
			log.Printf("%s has %d members\n", groupEmail, len(members))
			break
		}
		log.Printf("Members thus far %s --> [%d]\n", groupEmail, len(members))
	}
	return members
}
