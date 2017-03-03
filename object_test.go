package cloudstorage_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/api/iterator"

	"github.com/lytics/cloudstorage"
	"github.com/lytics/cloudstorage/testutils"
)

/*

# to use Google Cloud Storage ensure your current env
# has access to a google cloud storage account and then
# export TESTINT

export TESTINT=1

*/
func TestBasicRW(t *testing.T) {
	store := testutils.CreateStore(t)
	testutils.Clearstore(t, store)
	//
	//Create a new object and write to it.
	//
	obj, err := store.NewObject("prefix/test.csv")
	testutils.AssertEq(t, nil, err, "error.")
	testutils.AssertT(t, obj != nil, "Should return new empty obj")

	f, err := obj.Open(cloudstorage.ReadWrite)
	testutils.AssertEq(t, nil, err, "error.")
	testutils.AssertT(t, f != nil, "the file was nil")

	testcsv := "Year,Make,Model\n1997,Ford,E350\n2000,Mercury,Cougar\n"

	w := bufio.NewWriter(f)
	n, err := w.WriteString(testcsv)
	testutils.AssertEq(t, nil, err, "error. %d", n)
	w.Flush()

	err = obj.Close()
	testutils.AssertEq(t, nil, err, "error.")

	//
	//Read the object back out of the cloud storage.
	//
	obj2, err := store.Get("prefix/test.csv")
	testutils.AssertEq(t, nil, err, "error.")

	f2, err := obj2.Open(cloudstorage.ReadOnly)
	testutils.AssertEq(t, nil, err, "error.")

	bytes, err := ioutil.ReadAll(f2)
	testutils.AssertEq(t, nil, err, "error.")

	testutils.AssertEq(t, testcsv, string(bytes))
}

func TestAppend(t *testing.T) {
	store := testutils.CreateStore(t)
	testutils.Clearstore(t, store)
	now := time.Now()
	time.Sleep(10 * time.Millisecond)

	//
	//Create a new object and write to it.
	//
	obj, err := store.NewObject("test.csv")
	testutils.AssertEq(t, nil, err, "error.")

	f1, err := obj.Open(cloudstorage.ReadWrite)
	testutils.AssertEq(t, nil, err, "error.")
	testutils.AssertT(t, f1 != nil, "the file was nil")

	testcsv := "Year,Make,Model\n2003,VW,EuroVan\n2001,Ford,Ranger\n"

	w1 := bufio.NewWriter(f1)
	n1, err := w1.WriteString(testcsv)
	testutils.AssertEq(t, nil, err, "error. %d", n1)
	w1.Flush()

	err = obj.Close()
	testutils.AssertEq(t, nil, err, "error.")

	//
	// get the object and append to it...
	//
	morerows := "2013,VW,Jetta\n2011,Dodge,Caravan\n"
	obj2, err := store.Get("test.csv")
	testutils.AssertEq(t, nil, err, "error.")

	// get updated time
	updated := obj2.Updated()
	testutils.AssertT(t, updated.After(now), "updated time was not set")
	time.Sleep(10 * time.Millisecond)

	f2, err := obj2.Open(cloudstorage.ReadWrite)
	testutils.AssertEq(t, nil, err, "error.")
	testutils.AssertT(t, f2 != nil, "the file was nil")

	w2 := bufio.NewWriter(f2)
	n2, err := w2.WriteString(morerows)
	testutils.AssertEq(t, nil, err, "error. %d", n2)
	w2.Flush()

	err = obj2.Close()
	testutils.AssertEq(t, nil, err, "error.")

	//
	//Read the object back out of the cloud storage.
	//
	obj3, err := store.Get("test.csv")
	testutils.AssertEq(t, nil, err, "error.")
	updated3 := obj3.Updated()
	testutils.AssertT(t, updated3.After(updated), "updated time not updated")
	f3, err := obj3.Open(cloudstorage.ReadOnly)
	testutils.AssertEq(t, nil, err, "error.")

	bytes, err := ioutil.ReadAll(f3)
	testutils.AssertEq(t, nil, err, "error.")

	testutils.AssertEq(t, testcsv+morerows, string(bytes), "not the rows we expected.")
}

func TestListObjsAndFolders(t *testing.T) {
	store := testutils.CreateStore(t)
	testutils.Clearstore(t, store)
	//
	// Create 5 objects in each of 3 folders
	// ie 15 objects
	//
	folders := []string{"a", "b", "c"}
	names := []string{}
	for _, folder := range folders {
		for i := 0; i < 5; i++ {
			n := fmt.Sprintf("list-test/%s/test%d.csv", folder, i)
			names = append(names, n)
		}
	}

	sort.Strings(names)

	for _, n := range names {
		obj, err := store.NewObject(n)
		testutils.AssertEq(t, nil, err, "error. %v", obj)

		f1, err := obj.Open(cloudstorage.ReadWrite)
		testutils.AssertEq(t, nil, err, "error.")
		testutils.AssertT(t, f1 != nil, "the file was nil")

		testcsv := "12345\n"

		w1 := bufio.NewWriter(f1)
		n1, err := w1.WriteString(testcsv)
		testutils.AssertEq(t, nil, err, "error. %d", n1)
		w1.Flush()

		err = obj.Close()
		testutils.AssertEq(t, nil, err, "error.")
	}

	q := cloudstorage.NewQuery("list-test/")
	q.Sorted()
	objs, err := store.List(q)
	testutils.AssertEq(t, nil, err, "error.")

	testutils.AssertEq(t, 15, len(objs), "incorrect list len. wanted 15 got %d", len(objs))

	for i, o := range objs {
		t.Logf("%d found %v", i, o.Name())
		testutils.AssertEq(t, names[i], o.Name(), "unexpected name.")
	}

	// Now with iterator
	iter := store.Objects(context.Background(), q)

	objs = make(cloudstorage.Objects, 0)
	i := 0
	for {
		o, err := iter.Next()
		if err == iterator.Done {
			break
		}
		objs = append(objs, o)
		t.Logf("%d found %v", i, o.Name())
		testutils.AssertEq(t, names[i], o.Name(), "unexpected name.")
		i++
	}

	testutils.AssertEq(t, 15, len(objs), "incorrect list len.")

	q = cloudstorage.NewQueryForFolders("list-test/")
	folders, err = store.Folders(context.Background(), q)
	t.Logf("folders %v", folders)
	testutils.AssertEq(t, nil, err, "error.")
	testutils.AssertEq(t, 3, len(folders), "incorrect list len. wanted 3 folders. ", folders)
}

func TestTruncate(t *testing.T) {
	store := testutils.CreateStore(t)
	testutils.Clearstore(t, store)
	//
	//Create a new object and write to it.
	//
	obj, err := store.NewObject("test.csv")
	testutils.AssertEq(t, nil, err, "error.")

	f1, err := obj.Open(cloudstorage.ReadWrite)
	testutils.AssertEq(t, nil, err, "error.")
	testutils.AssertT(t, f1 != nil, "the file was nil")

	testcsv := "Year,Make,Model\n2003,VW,EuroVan\n2001,Ford,Ranger\n"

	w1 := bufio.NewWriter(f1)
	n1, err := w1.WriteString(testcsv)
	testutils.AssertEq(t, nil, err, "error. %d", n1)
	w1.Flush()

	err = obj.Close()
	testutils.AssertEq(t, nil, err, "error.")

	//
	//get the object and replace it...
	//
	newtestcsv := "Year,Make,Model\n2013,VW,Jetta\n"
	obj2, err := store.Get("test.csv")
	testutils.AssertEq(t, nil, err, "error.")

	f2, err := obj2.Open(cloudstorage.ReadWrite)
	testutils.AssertEq(t, nil, err, "error.")
	testutils.AssertT(t, f2 != nil, "the file was nil")

	// Truncating the file will zero out the file
	f2.Truncate(0)
	// We also want to start writing from the beginning of the file
	f2.Seek(0, 0)

	w2 := bufio.NewWriter(f2)
	n2, err := w2.WriteString(newtestcsv)
	testutils.AssertEq(t, nil, err, "error. %d", n2)
	w2.Flush()

	err = obj2.Close()
	testutils.AssertEq(t, nil, err, "error.")

	//
	// Read the object back out of the cloud storage.
	//
	obj3, err := store.Get("test.csv")
	testutils.AssertEq(t, nil, err, "error.")

	f3, err := obj3.Open(cloudstorage.ReadOnly)
	testutils.AssertEq(t, nil, err, "error.")

	bytes, err := ioutil.ReadAll(f3)
	testutils.AssertEq(t, nil, err, "error.")

	testutils.AssertEq(t, newtestcsv, string(bytes), "not the rows we expected.")
}

func TestNewObjectWithExisting(t *testing.T) {
	store := testutils.CreateStore(t)
	testutils.Clearstore(t, store)
	//
	//Create a new object and write to it.
	//
	obj, err := store.NewObject("test.csv")
	testutils.AssertEq(t, nil, err, "error.")

	f, err := obj.Open(cloudstorage.ReadWrite)
	testutils.AssertEq(t, nil, err, "error.")
	testutils.AssertT(t, f != nil, "the file was nil")

	testcsv := "Year,Make,Model\n2003,VW,EuroVan\n2001,Ford,Ranger\n"

	w := bufio.NewWriter(f)
	n, err := w.WriteString(testcsv)
	testutils.AssertEq(t, nil, err, "error. %d", n)
	w.Flush()

	err = obj.Close()
	testutils.AssertEq(t, nil, err, "error.")

	//
	// Ensure calling NewObject on an existing object returns an error,
	// because the object exits.
	//
	obj2, err := store.NewObject("test.csv")
	testutils.AssertEq(t, cloudstorage.ObjectExists, err, "error.")
	testutils.AssertEq(t, nil, obj2, "object shoudl be nil.")

	//
	//Read the object back out of the cloud storage.
	//
	obj3, err := store.Get("test.csv")
	testutils.AssertEq(t, nil, err, "error.")

	f3, err := obj3.Open(cloudstorage.ReadOnly)
	testutils.AssertEq(t, nil, err, "error.")

	bytes, err := ioutil.ReadAll(f3)
	testutils.AssertEq(t, nil, err, "error.")

	testutils.AssertEq(t, testcsv, string(bytes))
}

func TestReadWriteCloser(t *testing.T) {
	store := testutils.CreateStore(t)
	testutils.Clearstore(t, store)

	object := "prefix/iorw.test"
	data := fmt.Sprintf("pid:%v:time:%v", os.Getpid(), time.Now().Nanosecond())

	wc, err := store.NewWriter(object, nil)
	testutils.AssertEq(t, nil, err, "error.")
	buf1 := bytes.NewBufferString(data)
	_, err = buf1.WriteTo(wc)
	testutils.AssertEq(t, nil, err, "error.")
	err = wc.Close()
	testutils.AssertEq(t, nil, err, "error.")

	rc, err := store.NewReader(object)
	testutils.AssertEq(t, nil, err, "error.")
	buf2 := bytes.Buffer{}
	_, err = buf2.ReadFrom(rc)
	testutils.AssertEq(t, nil, err, "error.")
	testutils.AssertEq(t, data, buf2.String(), "round trip data don't match")

}
